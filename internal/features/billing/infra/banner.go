package infra

import (
	"context"
	"fmt"

	moderation "github.com/hayakawakaki/go-racp/internal/features/account/app/moderation"
	"github.com/hayakawakaki/go-racp/internal/features/billing/app"
)

var _ app.AccountBanner = (*ChargebackBanner)(nil)

type ChargebackBanner struct {
	svc *moderation.Service
}

func NewChargebackBanner(svc *moderation.Service) *ChargebackBanner {
	return &ChargebackBanner{svc: svc}
}

func (b *ChargebackBanner) BanForChargeback(ctx context.Context, accountID int, reason string) error {
	if err := b.svc.BanForChargeback(ctx, accountID, reason); err != nil {
		return fmt.Errorf("infra.ChargebackBanner.BanForChargeback: %w", err)
	}

	return nil
}
