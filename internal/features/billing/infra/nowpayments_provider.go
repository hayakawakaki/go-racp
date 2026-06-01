package infra

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

var _ domain.Provider = (*NowPaymentsProvider)(nil)

type NowPaymentsProvider struct {
	client         *NowPaymentsClient
	ipnCallbackURL string
}

func NewNowPaymentsProvider(client *NowPaymentsClient, ipnCallbackURL string) *NowPaymentsProvider {
	return &NowPaymentsProvider{client: client, ipnCallbackURL: ipnCallbackURL}
}

func (p *NowPaymentsProvider) Name() string { return "crypto" }

func (p *NowPaymentsProvider) CreateCheckout(ctx context.Context, request domain.CheckoutRequest) (domain.CheckoutResult, error) {
	invoice, err := p.client.CreateInvoice(ctx, CreateInvoiceParams{
		PriceAmount:      request.Amount,
		PriceCurrency:    strings.ToLower(request.Currency),
		OrderID:          strconv.FormatInt(request.PurchaseID, 10),
		OrderDescription: request.Description,
		SuccessURL:       request.SuccessURL,
		CancelURL:        request.CancelURL,
		IPNCallbackURL:   p.ipnCallbackURL,
	})
	if err != nil {
		return domain.CheckoutResult{}, fmt.Errorf("billing.nowpayments.CreateCheckout: %w", err)
	}

	return domain.CheckoutResult{RedirectURL: invoice.InvoiceURL, Reference: invoice.InvoiceID}, nil
}

func (p *NowPaymentsProvider) RetrieveCheckout(ctx context.Context, values url.Values) (domain.CheckoutConfirmation, error) {
	return domain.CheckoutConfirmation{}, nil
}
