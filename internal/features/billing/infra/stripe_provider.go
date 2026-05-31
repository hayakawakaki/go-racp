package infra

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/checkout/session"
)

var _ domain.Provider = (*StripeProvider)(nil)

type StripeProvider struct{}

func NewStripeProvider(secretKey string) *StripeProvider {
	stripe.Key = secretKey

	return &StripeProvider{}
}

func (p *StripeProvider) Name() string { return "stripe" }

func (p *StripeProvider) CreateCheckout(ctx context.Context, request domain.CheckoutRequest) (domain.CheckoutResult, error) {
	unitAmountMinor, err := domain.ToMinorUnits(request.Amount, request.Currency)
	if err != nil {
		return domain.CheckoutResult{}, fmt.Errorf("billing.stripe.CreateCheckout: %w", err)
	}

	purchaseID := strconv.FormatInt(request.PurchaseID, 10)
	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL: stripe.String(request.SuccessURL),
		CancelURL:  stripe.String(request.CancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String(request.Currency),
					UnitAmount: new(unitAmountMinor),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(request.Description),
					},
				},
			},
		},
	}
	params.Context = ctx
	params.AddMetadata("purchase_id", purchaseID)

	created, err := session.New(params)
	if err != nil {
		return domain.CheckoutResult{}, fmt.Errorf("billing.stripe.CreateCheckout: %w", err)
	}

	return domain.CheckoutResult{RedirectURL: created.URL, Reference: created.ID}, nil
}
