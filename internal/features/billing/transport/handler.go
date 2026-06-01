package transport

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/hayakawakaki/go-racp/internal/features/billing/domain"
	"github.com/hayakawakaki/go-racp/internal/features/billing/infra"
	"github.com/hayakawakaki/go-racp/internal/features/billing/transport/state"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	"github.com/hayakawakaki/go-racp/internal/platform/routes"
	"github.com/hayakawakaki/go-racp/server/config"
)

const (
	maxCheckoutFormBytes = 1 << 10

	fieldPackage  = "package"
	fieldProvider = "provider"

	providerStripe = "stripe"
	providerCrypto = "crypto"

	historyPageSize = 50
)

type billingFulfiller interface {
	CompletePurchase(ctx context.Context, purchaseID int64, providerPaymentID string) error
	RefundPurchase(ctx context.Context, provider, providerPaymentID string) error
	DisputePurchase(ctx context.Context, provider, providerPaymentID string) error
	FailPurchase(ctx context.Context, purchaseID int64) error
}

type paypalVerifier interface {
	VerifyWebhook(ctx context.Context, params infra.WebhookSignatureParams) (bool, error)
}

type nowpaymentsVerifier interface {
	VerifyIPN(signature string, body []byte) (bool, error)
}

type billingService interface {
	Packages() []domain.Package
	Available() bool
	ProviderEnabled(key string) bool
	StartCheckout(ctx context.Context, accountID int, providerKey, packageKey, successURL, cancelURL string) (string, error)
	HistoryByAccount(ctx context.Context, accountID, limit int) ([]domain.Purchase, error)
	ConfirmCheckout(ctx context.Context, providerKey string, values url.Values, accountID int) (domain.Package, bool, error)
	CaptureApprovedOrder(ctx context.Context, providerKey, reference string, purchaseID int64) error
	billingFulfiller
}

type Renderer interface {
	StorePage(layout httpx.Layout, state state.StoreState) templ.Component
	PurchaseHistoryPage(layout httpx.Layout, state state.PurchaseHistoryState) templ.Component
	PurchaseHistoryContent(state state.PurchaseHistoryState) templ.Component
	PurchaseHistorySummary(state state.PurchaseHistoryState) templ.Component
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type HandlerConfig struct {
	Logger              *slog.Logger
	Theme               Renderer
	Paypal              paypalVerifier
	NowPayments         nowpaymentsVerifier
	Currency            string
	AppURL              string
	StripeWebhookSecret string
	PaypalWebhookID     string
	General             config.GeneralConfig
}

//nolint:govet // GeneralConfig trailing bool forces alignment cost
type Handler struct {
	svc                 billingService
	theme               Renderer
	logger              *slog.Logger
	paypal              paypalVerifier
	nowpayments         nowpaymentsVerifier
	currency            string
	appURL              string
	stripeWebhookSecret string
	paypalWebhookID     string
	general             config.GeneralConfig
}

func NewHandler(svc billingService, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		svc:                 svc,
		theme:               cfg.Theme,
		logger:              logger,
		paypal:              cfg.Paypal,
		nowpayments:         cfg.NowPayments,
		currency:            cfg.Currency,
		appURL:              cfg.AppURL,
		stripeWebhookSecret: cfg.StripeWebhookSecret,
		paypalWebhookID:     cfg.PaypalWebhookID,
		general:             cfg.General,
	}
}

func (h *Handler) layout() httpx.Layout {
	return httpx.Layout{GeneralConfig: h.general}
}

func (h *Handler) RegisterRoutes(reg *routes.Registry, mux *http.ServeMux) {
	reg.Wrap(mux, "Store.View", "GET /store", http.HandlerFunc(h.showStore))
	reg.Wrap(mux, "Store.Checkout", "POST /store/checkout", http.HandlerFunc(h.startCheckout))
	reg.Wrap(mux, "Store.History", "GET /store/history", http.HandlerFunc(h.showHistory))
	reg.Wrap(mux, "Store.History", "GET /store/history/summary", http.HandlerFunc(h.showHistorySummary))
	reg.Wrap(mux, "Webhooks.Stripe", "POST /webhooks/stripe", http.HandlerFunc(h.stripeWebhook))
	reg.Wrap(mux, "Webhooks.Paypal", "POST /webhooks/paypal", http.HandlerFunc(h.paypalWebhook))
	reg.Wrap(mux, "Webhooks.Crypto", "POST /webhooks/nowpayments", http.HandlerFunc(h.nowpaymentsWebhook))
}
