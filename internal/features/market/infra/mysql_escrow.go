package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

type EscrowRepository struct {
	Client   *sql.DB
	MaxSlots int
	MaxStack int
}

func NewEscrowRepository(client *sql.DB, maxSlots, maxStack int) *EscrowRepository {
	if maxSlots <= 0 {
		maxSlots = domain.DefaultMaxStorageSlots
	}
	if maxStack <= 0 || maxStack > domain.DefaultMaxStackAmount {
		maxStack = domain.DefaultMaxStackAmount
	}

	return &EscrowRepository{Client: client, MaxSlots: maxSlots, MaxStack: maxStack}
}

func requireLocked(ctx context.Context, tx *sql.Tx, accountID int) error {
	var locked bool
	err := tx.QueryRowContext(ctx,
		"SELECT is_locked FROM cp_storage_lock WHERE account_id = ? FOR UPDATE", accountID,
	).Scan(&locked)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrStorageUnlocked
	}
	if err != nil {
		return fmt.Errorf("infra.requireLocked: %w", err)
	}
	if !locked {
		return domain.ErrStorageUnlocked
	}

	return nil
}

func loadStashItemTx(ctx context.Context, tx *sql.Tx, accountID int, id int64) (domain.StashItem, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+itemColumns+" FROM cp_storage WHERE id = ? AND account_id = ? FOR UPDATE", id, accountID,
	)
	if err != nil {
		return domain.StashItem{}, fmt.Errorf("infra.loadStashItemTx query: %w", err)
	}

	items, err := collectRows(rows, scanStashItem)
	if err != nil {
		return domain.StashItem{}, fmt.Errorf("infra.loadStashItemTx: %w", err)
	}
	if len(items) == 0 {
		return domain.StashItem{}, domain.ErrStashItemNotFound
	}

	return items[0], nil
}

func insertEscrowRow(ctx context.Context, tx *sql.Tx, listingRef int64, item domain.StashItem, amount int) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO cp_storage_escrow
		 (account_id, listing_ref, nameid, amount, equip, identify, refine, attribute,
		  card0, card1, card2, card3,
		  option_id0, option_val0, option_parm0,
		  option_id1, option_val1, option_parm1,
		  option_id2, option_val2, option_parm2,
		  option_id3, option_val3, option_parm3,
		  option_id4, option_val4, option_parm4,
		  expire_time, bound, unique_id, enchantgrade)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.AccountID, listingRef, item.NameID, amount, item.Equip, item.Identify, item.Refine, item.Attribute,
		item.Card[0], item.Card[1], item.Card[2], item.Card[3],
		item.OptionID[0], item.OptionVal[0], item.OptionParm[0],
		item.OptionID[1], item.OptionVal[1], item.OptionParm[1],
		item.OptionID[2], item.OptionVal[2], item.OptionParm[2],
		item.OptionID[3], item.OptionVal[3], item.OptionParm[3],
		item.OptionID[4], item.OptionVal[4], item.OptionParm[4],
		item.ExpireTime, item.Bound, item.UniqueID, item.Grade,
	)
	if err != nil {
		return fmt.Errorf("infra.insertEscrowRow: %w", err)
	}

	return nil
}

func upsertStashStack(ctx context.Context, tx *sql.Tx, toAccountID int, item domain.StashItem, amount, maxSlots, maxStack int) error {
	remaining, err := mergeIntoExistingStack(ctx, tx, toAccountID, item, amount, maxStack)
	if err != nil {
		return err
	}

	return insertStashChunks(ctx, tx, toAccountID, item, remaining, maxSlots, maxStack)
}

func mergeIntoExistingStack(ctx context.Context, tx *sql.Tx, toAccountID int, item domain.StashItem, amount, maxStack int) (int, error) {
	if !item.IsStackable() {
		return amount, nil
	}

	var rowID int64
	var existing int
	err := tx.QueryRowContext(ctx,
		"SELECT id, amount FROM cp_storage WHERE "+stackableMatchWhere+" AND amount < ? ORDER BY id LIMIT 1 FOR UPDATE",
		toAccountID, item.NameID, item.Identify, item.ExpireTime, maxStack,
	).Scan(&rowID, &existing)
	if errors.Is(err, sql.ErrNoRows) {
		return amount, nil
	}
	if err != nil {
		return 0, fmt.Errorf("infra.mergeIntoExistingStack find: %w", err)
	}

	add := min(amount, maxStack-existing)
	if _, err := tx.ExecContext(ctx, "UPDATE cp_storage SET amount = amount + ? WHERE id = ?", add, rowID); err != nil {
		return 0, fmt.Errorf("infra.mergeIntoExistingStack merge: %w", err)
	}

	return amount - add, nil
}

func insertStashChunks(ctx context.Context, tx *sql.Tx, toAccountID int, item domain.StashItem, amount, maxSlots, maxStack int) error {
	for amount > 0 {
		var used int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM cp_storage WHERE account_id = ?", toAccountID).Scan(&used); err != nil {
			return fmt.Errorf("infra.insertStashChunks count: %w", err)
		}
		if used >= maxSlots {
			return domain.ErrStorageFull
		}

		chunk := amount
		if item.IsStackable() && chunk > maxStack {
			chunk = maxStack
		}

		item.AccountID = toAccountID
		if err := insertStashRow(ctx, tx, item, chunk); err != nil {
			return err
		}
		amount -= chunk
	}

	return nil
}

func insertStashRow(ctx context.Context, tx *sql.Tx, item domain.StashItem, amount int) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO cp_storage
		 (account_id, nameid, amount, equip, identify, refine, attribute,
		  card0, card1, card2, card3,
		  option_id0, option_val0, option_parm0,
		  option_id1, option_val1, option_parm1,
		  option_id2, option_val2, option_parm2,
		  option_id3, option_val3, option_parm3,
		  option_id4, option_val4, option_parm4,
		  expire_time, bound, unique_id, enchantgrade)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.AccountID, item.NameID, amount, item.Equip, item.Identify, item.Refine, item.Attribute,
		item.Card[0], item.Card[1], item.Card[2], item.Card[3],
		item.OptionID[0], item.OptionVal[0], item.OptionParm[0],
		item.OptionID[1], item.OptionVal[1], item.OptionParm[1],
		item.OptionID[2], item.OptionVal[2], item.OptionParm[2],
		item.OptionID[3], item.OptionVal[3], item.OptionParm[3],
		item.OptionID[4], item.OptionVal[4], item.OptionParm[4],
		item.ExpireTime, item.Bound, item.UniqueID, item.Grade,
	)
	if err != nil {
		return fmt.Errorf("infra.insertStashRow: %w", err)
	}

	return nil
}

func (r *EscrowRepository) MoveToEscrow(ctx context.Context, accountID int, listingRef int64, moves []domain.EscrowMove) error {
	tx, err := r.Client.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("infra.EscrowRepository.MoveToEscrow begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireLocked(ctx, tx, accountID); err != nil {
		return err
	}

	for _, move := range moves {
		if err := moveOneToEscrow(ctx, tx, accountID, listingRef, move); err != nil {
			return fmt.Errorf("infra.EscrowRepository.MoveToEscrow: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("infra.EscrowRepository.MoveToEscrow commit: %w", err)
	}

	return nil
}

func validateEscrowMove(item domain.StashItem, move domain.EscrowMove) error {
	if !item.IsTradable() {
		return domain.ErrNotTradable
	}
	if move.Amount != item.Amount && !item.IsStackable() {
		return domain.ErrNonStackable
	}
	if move.Amount <= 0 || move.Amount > item.Amount {
		return domain.ErrInsufficientStack
	}

	return nil
}

func moveOneToEscrow(ctx context.Context, tx *sql.Tx, accountID int, listingRef int64, move domain.EscrowMove) error {
	item, err := loadStashItemTx(ctx, tx, accountID, move.StashItemID)
	if err != nil {
		return fmt.Errorf("infra.moveOneToEscrow load: %w", err)
	}
	if err := validateEscrowMove(item, move); err != nil {
		return err
	}

	if err := insertEscrowRow(ctx, tx, listingRef, item, move.Amount); err != nil {
		return fmt.Errorf("infra.moveOneToEscrow insert: %w", err)
	}

	if move.Amount == item.Amount {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cp_storage WHERE id = ?", item.ID); err != nil {
			return fmt.Errorf("infra.moveOneToEscrow delete: %w", err)
		}

		return nil
	}

	if _, err := tx.ExecContext(ctx, "UPDATE cp_storage SET amount = amount - ? WHERE id = ?", move.Amount, item.ID); err != nil {
		return fmt.Errorf("infra.moveOneToEscrow reduce: %w", err)
	}

	return nil
}

func (r *EscrowRepository) drainEscrow(ctx context.Context, listingRef int64, toAccountID int, legID int64) error {
	tx, err := r.Client.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("infra.EscrowRepository.drainEscrow begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	applied, err := claimDelivery(ctx, tx, legID)
	if err != nil {
		return fmt.Errorf("infra.EscrowRepository.drainEscrow claim: %w", err)
	}
	if !applied {
		if err := drainEscrowTx(ctx, tx, listingRef, toAccountID, r.MaxSlots, r.MaxStack); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("infra.EscrowRepository.drainEscrow commit: %w", err)
	}

	return nil
}

func drainEscrowTx(ctx context.Context, tx *sql.Tx, listingRef int64, toAccountID, maxSlots, maxStack int) error {
	if err := requireLocked(ctx, tx, toAccountID); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx,
		"SELECT "+itemColumns+" FROM cp_storage_escrow WHERE listing_ref = ? FOR UPDATE", listingRef,
	)
	if err != nil {
		return fmt.Errorf("infra.drainEscrowTx query: %w", err)
	}

	items, err := collectRows(rows, scanStashItem)
	if err != nil {
		return fmt.Errorf("infra.drainEscrowTx collect: %w", err)
	}

	for _, item := range items {
		if err := upsertStashStack(ctx, tx, toAccountID, item, item.Amount, maxSlots, maxStack); err != nil {
			return fmt.Errorf("infra.drainEscrowTx deliver: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM cp_storage_escrow WHERE listing_ref = ?", listingRef); err != nil {
		return fmt.Errorf("infra.drainEscrowTx clear: %w", err)
	}

	return nil
}

func claimDelivery(ctx context.Context, tx *sql.Tx, legID int64) (alreadyApplied bool, err error) {
	if legID == 0 {
		return false, nil
	}

	res, err := tx.ExecContext(ctx, "INSERT IGNORE INTO cp_delivery_applied (leg_id) VALUES (?)", legID)
	if err != nil {
		return false, fmt.Errorf("infra.claimDelivery: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("infra.claimDelivery affected: %w", err)
	}

	return affected == 0, nil
}

func (r *EscrowRepository) ReturnToStash(ctx context.Context, listingRef int64) error {
	owner, err := r.escrowOwner(ctx, listingRef)
	if errors.Is(err, domain.ErrStashItemNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	return r.drainEscrow(ctx, listingRef, owner, 0)
}

func (r *EscrowRepository) Deliver(ctx context.Context, listingRef int64, toAccountID int, legID int64) error {
	return r.drainEscrow(ctx, listingRef, toAccountID, legID)
}

func (r *EscrowRepository) escrowOwner(ctx context.Context, listingRef int64) (int, error) {
	var owner int
	err := r.Client.QueryRowContext(ctx,
		"SELECT account_id FROM cp_storage_escrow WHERE listing_ref = ? LIMIT 1", listingRef,
	).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrStashItemNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("infra.EscrowRepository.escrowOwner: %w", err)
	}

	return owner, nil
}

func (r *EscrowRepository) ByListing(ctx context.Context, listingRef int64) ([]domain.StashItem, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT "+itemColumns+" FROM cp_storage_escrow WHERE listing_ref = ? ORDER BY id", listingRef,
	)
	if err != nil {
		return nil, fmt.Errorf("infra.EscrowRepository.ByListing: %w", err)
	}

	items, err := collectRows(rows, scanStashItem)
	if err != nil {
		return nil, fmt.Errorf("infra.EscrowRepository.ByListing: %w", err)
	}

	return items, nil
}

func (r *EscrowRepository) OrphanRefs(ctx context.Context, before time.Time) ([]int64, error) {
	rows, err := r.Client.QueryContext(ctx,
		"SELECT listing_ref FROM cp_storage_escrow GROUP BY listing_ref HAVING UNIX_TIMESTAMP(MAX(created_at)) < ?", before.Unix(),
	)
	if err != nil {
		return nil, fmt.Errorf("infra.EscrowRepository.OrphanRefs: %w", err)
	}

	refs, err := collectRows(rows, scanListingRef)
	if err != nil {
		return nil, fmt.Errorf("infra.EscrowRepository.OrphanRefs: %w", err)
	}

	return refs, nil
}

func scanListingRef(rows *sql.Rows) (int64, error) {
	var ref int64
	if err := rows.Scan(&ref); err != nil {
		return 0, fmt.Errorf("infra.scanListingRef: %w", err)
	}

	return ref, nil
}

func (r *EscrowRepository) DeliverPartial(ctx context.Context, listingRef int64, toAccountID, amount int, legID int64) error {
	tx, err := r.Client.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("infra.EscrowRepository.DeliverPartial begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	applied, err := claimDelivery(ctx, tx, legID)
	if err != nil {
		return fmt.Errorf("infra.EscrowRepository.DeliverPartial claim: %w", err)
	}
	if !applied {
		if err := deliverPartialTx(ctx, tx, listingRef, toAccountID, amount, r.MaxSlots, r.MaxStack); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("infra.EscrowRepository.DeliverPartial commit: %w", err)
	}

	return nil
}

func deliverPartialTx(ctx context.Context, tx *sql.Tx, listingRef int64, toAccountID, amount, maxSlots, maxStack int) error {
	if err := requireLocked(ctx, tx, toAccountID); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx,
		"SELECT "+itemColumns+" FROM cp_storage_escrow WHERE listing_ref = ? ORDER BY id LIMIT 1 FOR UPDATE", listingRef,
	)
	if err != nil {
		return fmt.Errorf("infra.deliverPartialTx query: %w", err)
	}

	items, err := collectRows(rows, scanStashItem)
	if err != nil {
		return fmt.Errorf("infra.deliverPartialTx collect: %w", err)
	}
	if len(items) == 0 {
		return domain.ErrStashItemNotFound
	}

	item := items[0]
	if amount <= 0 || amount > item.Amount {
		return domain.ErrInsufficientStack
	}

	if err := upsertStashStack(ctx, tx, toAccountID, item, amount, maxSlots, maxStack); err != nil {
		return fmt.Errorf("infra.deliverPartialTx deliver: %w", err)
	}

	return reduceEscrowRow(ctx, tx, item, amount)
}

func reduceEscrowRow(ctx context.Context, tx *sql.Tx, item domain.StashItem, amount int) error {
	if amount == item.Amount {
		if _, err := tx.ExecContext(ctx, "DELETE FROM cp_storage_escrow WHERE id = ?", item.ID); err != nil {
			return fmt.Errorf("infra.reduceEscrowRow delete: %w", err)
		}

		return nil
	}

	if _, err := tx.ExecContext(ctx, "UPDATE cp_storage_escrow SET amount = amount - ? WHERE id = ?", amount, item.ID); err != nil {
		return fmt.Errorf("infra.reduceEscrowRow reduce: %w", err)
	}

	return nil
}
