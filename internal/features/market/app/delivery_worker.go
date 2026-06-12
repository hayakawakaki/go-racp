package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

const reconcileGrace = 5 * time.Minute

type RefSource interface {
	AllRefs(ctx context.Context) ([]int64, error)
}

type Workers struct {
	listings   domain.ListingRepository
	escrow     domain.EscrowRepository
	wallet     domain.WalletRepository
	settlement domain.SettlementRepository
	logger     *slog.Logger
	now        func() time.Time
	extraRefs  []RefSource
	batch      int
}

type WorkerOption func(*Workers)

func WithRefSources(sources ...RefSource) WorkerOption {
	return func(w *Workers) { w.extraRefs = append(w.extraRefs, sources...) }
}

func NewWorkers(
	listings domain.ListingRepository,
	escrow domain.EscrowRepository,
	wallet domain.WalletRepository,
	settlement domain.SettlementRepository,
	logger *slog.Logger,
	opts ...WorkerOption,
) *Workers {
	w := &Workers{
		listings:   listings,
		escrow:     escrow,
		wallet:     wallet,
		settlement: settlement,
		logger:     logger,
		now:        time.Now,
		batch:      100,
	}
	for _, opt := range opts {
		opt(w)
	}

	return w
}

func (w *Workers) Deliver(ctx context.Context) (int64, error) {
	legs, err := w.settlement.Pending(ctx, w.batch)
	if err != nil {
		return 0, fmt.Errorf("app.Workers.Deliver: %w", err)
	}

	var delivered int64
	for _, leg := range legs {
		if deliverErr := w.deliverLeg(ctx, leg); deliverErr != nil {
			if errors.Is(deliverErr, domain.ErrStorageUnlocked) || errors.Is(deliverErr, domain.ErrStorageFull) {
				continue
			}
			w.logger.Error("market: deliver leg", "leg", leg.ID, "err", deliverErr)
			continue
		}
		if doneErr := w.settlement.MarkDone(ctx, leg.ID); doneErr != nil {
			w.logger.Error("market: mark settlement done", "leg", leg.ID, "err", doneErr)
			continue
		}
		delivered++
	}

	return delivered, nil
}

func (w *Workers) deliverLeg(ctx context.Context, leg domain.SettlementLeg) error {
	if leg.Whole {
		if err := w.escrow.Deliver(ctx, leg.EscrowRef, leg.RecipientAccountID, leg.ID); err != nil {
			return fmt.Errorf("app.Workers.deliverLeg whole: %w", err)
		}

		return nil
	}

	if err := w.escrow.DeliverPartial(ctx, leg.EscrowRef, leg.RecipientAccountID, leg.DeliverAmount, leg.ID); err != nil {
		return fmt.Errorf("app.Workers.deliverLeg partial: %w", err)
	}

	return nil
}

func (w *Workers) Reconcile(ctx context.Context) (int64, error) {
	orphans, err := w.escrow.OrphanRefs(ctx, w.now().Add(-reconcileGrace))
	if err != nil {
		return 0, fmt.Errorf("app.Workers.Reconcile orphans: %w", err)
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	known, err := w.knownRefs(ctx)
	if err != nil {
		return 0, fmt.Errorf("app.Workers.Reconcile known: %w", err)
	}

	var returned int64
	for _, ref := range orphans {
		if _, ok := known[ref]; ok {
			continue
		}
		if w.returnOrphan(ctx, ref) {
			returned++
		}
	}

	return returned, nil
}

func (w *Workers) knownRefs(ctx context.Context) (map[int64]struct{}, error) {
	known := make(map[int64]struct{})
	sources := append([]RefSource{w.listings}, w.extraRefs...)
	for _, source := range sources {
		refs, err := source.AllRefs(ctx)
		if err != nil {
			return nil, fmt.Errorf("app.Workers.knownRefs sources: %w", err)
		}
		for _, ref := range refs {
			known[ref] = struct{}{}
		}
	}

	pending, err := w.settlement.PendingRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("app.Workers.knownRefs pending: %w", err)
	}
	for _, ref := range pending {
		known[ref] = struct{}{}
	}

	return known, nil
}

func (w *Workers) returnOrphan(ctx context.Context, ref int64) bool {
	if err := w.escrow.ReturnToStash(ctx, ref); err != nil {
		if errors.Is(err, domain.ErrStorageUnlocked) {
			return false
		}
		w.logger.Error("market: reconcile return", "ref", ref, "err", err)

		return false
	}

	return true
}

func (w *Workers) Expire(ctx context.Context) (int64, error) {
	due, err := w.listings.DueForExpiry(ctx, w.now(), w.batch)
	if err != nil {
		return 0, fmt.Errorf("app.Workers.Expire: %w", err)
	}

	var expired int64
	for _, listing := range due {
		if statusErr := w.listings.SetStatus(ctx, listing.ID, domain.StatusExpired); statusErr != nil {
			w.logger.Error("market: expire status", "listing", listing.ID, "err", statusErr)
			continue
		}
		if listing.GiveItem {
			if enqueueErr := w.settlement.Enqueue(ctx, domain.SettlementLeg{
				ListingID:          listing.ID,
				EscrowRef:          listing.ID,
				RecipientAccountID: listing.SellerAccountID,
				DeliverAmount:      listing.GiveUnitAmount * listing.RemainingQuantity,
				Whole:              !listing.Stackable,
			}); enqueueErr != nil {
				w.logger.Error("market: expire enqueue", "listing", listing.ID, "err", enqueueErr)
			}
		}
		if listing.GiveHoldID != nil {
			if releaseErr := w.wallet.Release(ctx, *listing.GiveHoldID); releaseErr != nil {
				w.logger.Error("market: expire release", "listing", listing.ID, "err", releaseErr)
			}
		}
		expired++
	}

	return expired, nil
}
