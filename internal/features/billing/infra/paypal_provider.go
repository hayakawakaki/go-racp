package infra

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
)

var _ domain.Provider = (*PaypalProvider)(nil)

type PaypalProvider struct {
	client *PaypalClient
}

func NewPaypalProvider(client *PaypalClient) *PaypalProvider {
	return &PaypalProvider{client: client}
}

func (p *PaypalProvider) Name() string { return "paypal" }

func (p *PaypalProvider) CreateCheckout(ctx context.Context, request domain.CheckoutRequest) (domain.CheckoutResult, error) {
	value, err := domain.ToDecimalString(request.Amount, request.Currency)
	if err != nil {
		return domain.CheckoutResult{}, fmt.Errorf("billing.paypal.CreateCheckout: %w", err)
	}

	order, err := p.client.CreateOrder(ctx, CreateOrderParams{
		ReferenceID:  strconv.FormatInt(request.PurchaseID, 10),
		Description:  request.Description,
		CurrencyCode: request.Currency,
		Value:        value,
		ReturnURL:    request.SuccessURL,
		CancelURL:    request.CancelURL,
	})
	if err != nil {
		return domain.CheckoutResult{}, fmt.Errorf("billing.paypal.CreateCheckout: %w", err)
	}

	return domain.CheckoutResult{RedirectURL: order.ApprovalURL, Reference: order.OrderID}, nil
}

func (p *PaypalProvider) RetrieveCheckout(ctx context.Context, values url.Values) (domain.CheckoutConfirmation, error) {
	orderID := values.Get("token")
	if orderID == "" {
		return domain.CheckoutConfirmation{}, nil
	}

	details, err := p.client.GetOrder(ctx, orderID)
	if err != nil {
		return domain.CheckoutConfirmation{}, fmt.Errorf("billing.paypal.RetrieveCheckout: %w", err)
	}

	purchaseID, err := strconv.ParseInt(details.ReferenceID, 10, 64)
	if err != nil {
		return domain.CheckoutConfirmation{}, fmt.Errorf("billing.paypal.RetrieveCheckout: invalid reference id: %w", err)
	}

	return domain.CheckoutConfirmation{
		PurchaseID: purchaseID,
		Paid:       details.Status == "APPROVED" || details.Status == "COMPLETED",
	}, nil
}
