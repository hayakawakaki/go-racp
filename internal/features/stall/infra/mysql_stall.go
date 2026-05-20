package infra

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/features/stall/domain"
)

type Repository struct {
	Client *sql.DB
}

func NewRepository(client *sql.DB) *Repository {
	return &Repository{Client: client}
}

func (r *Repository) LoadAll(ctx context.Context) ([]domain.Vendor, error) {
	tx, err := r.Client.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("infra.Repository.LoadAll begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	selling, err := loadVendings(ctx, tx)
	if err != nil {
		return nil, err
	}
	if loadErr := loadVendingItems(ctx, tx, selling); loadErr != nil {
		return nil, loadErr
	}

	buying, err := loadBuyingstores(ctx, tx)
	if err != nil {
		return nil, err
	}
	if loadErr := loadBuyingstoreItems(ctx, tx, buying); loadErr != nil {
		return nil, loadErr
	}

	names, err := loadSellerNames(ctx, tx, selling, buying)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Vendor, 0, len(selling)+len(buying))
	for _, v := range selling {
		v.SellerName = names[v.CharID]
		out = append(out, *v)
	}
	for _, v := range buying {
		v.SellerName = names[v.CharID]
		out = append(out, *v)
	}

	return out, nil
}

func loadVendings(ctx context.Context, tx *sql.Tx) (map[int]*domain.Vendor, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, char_id, title, map, x, y, autotrade FROM vendings")
	if err != nil {
		return nil, fmt.Errorf("infra.loadVendings query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[int]*domain.Vendor{}
	for rows.Next() {
		var v domain.Vendor
		if err := rows.Scan(&v.ID, &v.CharID, &v.StallName, &v.VendorMap, &v.X, &v.Y, &v.Autotrade); err != nil {
			return nil, fmt.Errorf("infra.loadVendings scan: %w", err)
		}
		v.Type = domain.VendorTypeSelling
		out[v.ID] = &v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.loadVendings rows: %w", err)
	}

	return out, nil
}

func loadVendingItems(ctx context.Context, tx *sql.Tx, vendors map[int]*domain.Vendor) error {
	rows, err := tx.QueryContext(ctx,
		"SELECT vi.vending_id, vi.`index`, ci.nameid, vi.amount, vi.price "+
			"FROM vending_items vi "+
			"JOIN cart_inventory ci ON ci.id = vi.cartinventory_id",
	)
	if err != nil {
		return fmt.Errorf("infra.loadVendingItems query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			vendingID int
			item      domain.VendorItem
		)
		if err := rows.Scan(&vendingID, &item.Index, &item.ItemID, &item.Amount, &item.Price); err != nil {
			return fmt.Errorf("infra.loadVendingItems scan: %w", err)
		}
		if v, ok := vendors[vendingID]; ok {
			v.Items = append(v.Items, item)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("infra.loadVendingItems rows: %w", err)
	}

	return nil
}

func loadBuyingstores(ctx context.Context, tx *sql.Tx) (map[int]*domain.Vendor, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, char_id, title, map, x, y, autotrade, `limit` FROM buyingstores")
	if err != nil {
		return nil, fmt.Errorf("infra.loadBuyingstores query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := map[int]*domain.Vendor{}
	for rows.Next() {
		var v domain.Vendor
		if err := rows.Scan(&v.ID, &v.CharID, &v.StallName, &v.VendorMap, &v.X, &v.Y, &v.Autotrade, &v.BudgetLimit); err != nil {
			return nil, fmt.Errorf("infra.loadBuyingstores scan: %w", err)
		}
		v.Type = domain.VendorTypeBuying
		out[v.ID] = &v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.loadBuyingstores rows: %w", err)
	}

	return out, nil
}

func loadBuyingstoreItems(ctx context.Context, tx *sql.Tx, vendors map[int]*domain.Vendor) error {
	rows, err := tx.QueryContext(ctx,
		"SELECT buyingstore_id, `index`, item_id, amount, price FROM buyingstore_items",
	)
	if err != nil {
		return fmt.Errorf("infra.loadBuyingstoreItems query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			buyingID int
			item     domain.VendorItem
		)
		if err := rows.Scan(&buyingID, &item.Index, &item.ItemID, &item.Amount, &item.Price); err != nil {
			return fmt.Errorf("infra.loadBuyingstoreItems scan: %w", err)
		}
		if v, ok := vendors[buyingID]; ok {
			v.Items = append(v.Items, item)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("infra.loadBuyingstoreItems rows: %w", err)
	}

	return nil
}

func loadSellerNames(ctx context.Context, tx *sql.Tx, selling, buying map[int]*domain.Vendor) (map[int]string, error) {
	charIDs := map[int]struct{}{}
	for _, v := range selling {
		charIDs[v.CharID] = struct{}{}
	}
	for _, v := range buying {
		charIDs[v.CharID] = struct{}{}
	}
	if len(charIDs) == 0 {
		return map[int]string{}, nil
	}

	placeholders := make([]string, 0, len(charIDs))
	args := make([]any, 0, len(charIDs))
	for id := range charIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := "SELECT char_id, name FROM `char` WHERE char_id IN (" + strings.Join(placeholders, ",") + ")" //nolint:gosec // placeholders are constant "?"
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("infra.loadSellerNames query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	names := map[int]string{}
	for rows.Next() {
		var (
			charID int
			name   string
		)
		if err := rows.Scan(&charID, &name); err != nil {
			return nil, fmt.Errorf("infra.loadSellerNames scan: %w", err)
		}
		names[charID] = name
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("infra.loadSellerNames rows: %w", err)
	}

	return names, nil
}
