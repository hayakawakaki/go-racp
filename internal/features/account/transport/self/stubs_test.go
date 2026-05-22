package self

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/a-h/templ"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
	"github.com/hayakawakaki/go-racp/internal/features/account/transport/middleware"
	selfstate "github.com/hayakawakaki/go-racp/internal/features/account/transport/self/state"
	characterapp "github.com/hayakawakaki/go-racp/internal/features/character/app"
	actiontokendomain "github.com/hayakawakaki/go-racp/internal/platform/actiontoken/domain"
	"github.com/hayakawakaki/go-racp/internal/platform/httpx"
	accountself "github.com/hayakawakaki/go-racp/themes/default/features/account/transport/self"
	_ "github.com/hayakawakaki/go-racp/themes/default/platform/httpx"
)

const stubSessionTTL = 24 * time.Hour

type stubTheme struct{}

func (stubTheme) AccountPage(layout httpx.Layout, state selfstate.AccountState) templ.Component {
	return accountself.AccountPage(layout, state)
}
func (stubTheme) AccountChangeEmailModal(state selfstate.ChangeEmailState) templ.Component {
	return accountself.AccountChangeEmailModal(state)
}
func (stubTheme) AccountChangeEmailForm(state selfstate.ChangeEmailState) templ.Component {
	return accountself.AccountChangeEmailForm(state)
}
func (stubTheme) AccountChangeEmailPage(layout httpx.Layout, state selfstate.ChangeEmailState) templ.Component {
	return accountself.AccountChangeEmailPage(layout, state)
}
func (stubTheme) AccountChangePasswordModal(state selfstate.ChangePasswordState) templ.Component {
	return accountself.AccountChangePasswordModal(state)
}
func (stubTheme) AccountChangePasswordForm(state selfstate.ChangePasswordState) templ.Component {
	return accountself.AccountChangePasswordForm(state)
}
func (stubTheme) AccountChangePasswordPage(layout httpx.Layout, state selfstate.ChangePasswordState) templ.Component {
	return accountself.AccountChangePasswordPage(layout, state)
}
func (stubTheme) AccountEmailChangeResultPage(layout httpx.Layout, state selfstate.EmailChangeResultState) templ.Component {
	return accountself.AccountEmailChangeResultPage(layout, state)
}
func (stubTheme) AccountForgotPasswordPage(layout httpx.Layout, state selfstate.ForgotPasswordState) templ.Component {
	return accountself.AccountForgotPasswordPage(layout, state)
}
func (stubTheme) AccountForgotPasswordForm(state selfstate.ForgotPasswordState) templ.Component {
	return accountself.AccountForgotPasswordForm(state)
}
func (stubTheme) AccountLoginPage(layout httpx.Layout, state selfstate.LoginFormState) templ.Component {
	return accountself.AccountLoginPage(layout, state)
}
func (stubTheme) AccountLoginForm(state selfstate.LoginFormState) templ.Component {
	return accountself.AccountLoginForm(state)
}
func (stubTheme) AccountRegisterPage(layout httpx.Layout, state selfstate.RegisterFormState) templ.Component {
	return accountself.AccountRegisterPage(layout, state)
}
func (stubTheme) AccountRegisterForm(state selfstate.RegisterFormState) templ.Component {
	return accountself.AccountRegisterForm(state)
}
func (stubTheme) AccountResetPasswordPage(layout httpx.Layout, state selfstate.ResetPasswordState) templ.Component {
	return accountself.AccountResetPasswordPage(layout, state)
}
func (stubTheme) AccountResetResultPage(layout httpx.Layout, state selfstate.ResetResultState) templ.Component {
	return accountself.AccountResetResultPage(layout, state)
}
func (stubTheme) AccountVerifyAccountPage(layout httpx.Layout, state selfstate.VerifyAccountState) templ.Component {
	return accountself.AccountVerifyAccountPage(layout, state)
}
func (stubTheme) AccountVerifyConfirmPage(layout httpx.Layout, state selfstate.VerifyConfirmState) templ.Component {
	return accountself.AccountVerifyConfirmPage(layout, state)
}
func (stubTheme) AccountVerifyEmailChangeConfirmPage(layout httpx.Layout, state selfstate.VerifyEmailChangeConfirmState) templ.Component {
	return accountself.AccountVerifyEmailChangeConfirmPage(layout, state)
}
func (stubTheme) AccountVerifyResultPage(layout httpx.Layout, state selfstate.VerifyResultState) templ.Component {
	return accountself.AccountVerifyResultPage(layout, state)
}

func newTestHandler(svc accountService, sess sessionService, logBuffer io.Writer) *Handler {
	if logBuffer == nil {
		logBuffer = io.Discard
	}

	return &Handler{
		svc:        svc,
		sessSvc:    sess,
		theme:      stubTheme{},
		logger:     slog.New(slog.NewTextHandler(logBuffer, nil)),
		characters: &stubCharacterLister{},
	}
}

func reqWithSession(method, target string, userID int, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	ctx := middleware.ContextWithSession(req.Context(), &domain.Session{UserID: userID})
	return req.WithContext(ctx)
}

func postWithSession(target string, userID int, values map[string]string) *http.Request {
	req := reqWithSession(http.MethodPost, target, userID, strings.NewReader(encodeForm(values)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func postForm(target string, values map[string]string) *http.Request {
	form := strings.NewReader(encodeForm(values))
	req := httptest.NewRequest(http.MethodPost, target, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func encodeForm(v map[string]string) string {
	parts := make([]string, 0, len(v))
	for k, val := range v {
		parts = append(parts, k+"="+val)
	}
	return strings.Join(parts, "&")
}

type stubAccountService struct {
	createFn              func(context.Context, app.CreateCommand) (*app.GetDTO, error)
	authNFn               func(context.Context, app.LoginCommand) (*app.GetDTO, app.Tier, error)
	getAccountFn          func(context.Context, int) (*app.AccountDTO, error)
	issueVerificationFn   func(context.Context, int, string, string) error
	consumeVerificationFn func(context.Context, string) error
	resendVerificationFn  func(context.Context, int) error
	requestResetFn        func(context.Context, string) error
	consumeResetFn        func(context.Context, string, string) error
	peekFn                func(context.Context, actiontokendomain.Action, string) (*actiontokendomain.ActionToken, error)
	updatePasswordFn      func(context.Context, int, string, string, string, string) error
	requestEmailChangeFn  func(context.Context, int, string, string) error
	consumeEmailChangeFn  func(context.Context, string) (*domain.User, error)
	updatePasswordCalls   []updatePasswordCall
	requestEmailCalls     []requestEmailCall
}

type updatePasswordCall struct {
	CurrentRawToken string
	CurrentPassword string
	NewPassword     string
	ConfirmPassword string
	UserID          int
}

type requestEmailCall struct {
	CurrentPassword string
	NewEmail        string
	UserID          int
}

func (s *stubAccountService) Now() time.Time {
	return time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
}

func (s *stubAccountService) Create(ctx context.Context, cmd app.CreateCommand) (*app.GetDTO, error) {
	if s.createFn != nil {
		return s.createFn(ctx, cmd)
	}
	return &app.GetDTO{ID: 1, Username: cmd.Username, Email: cmd.Email}, nil
}

func (s *stubAccountService) Authenticate(ctx context.Context, cmd app.LoginCommand) (*app.GetDTO, app.Tier, error) {
	if s.authNFn != nil {
		return s.authNFn(ctx, cmd)
	}
	return &app.GetDTO{ID: 1, Username: cmd.Username}, app.TierActive, nil
}

func (s *stubAccountService) GetAccount(ctx context.Context, userID int) (*app.AccountDTO, error) {
	if s.getAccountFn != nil {
		return s.getAccountFn(ctx, userID)
	}
	return &app.AccountDTO{Username: "u", Email: "u@x", Verified: true}, nil
}

func (s *stubAccountService) IssueVerification(ctx context.Context, accountID int, email, username string) error {
	if s.issueVerificationFn != nil {
		return s.issueVerificationFn(ctx, accountID, email, username)
	}
	return nil
}

func (s *stubAccountService) ConsumeVerification(ctx context.Context, rawToken string) error {
	if s.consumeVerificationFn != nil {
		return s.consumeVerificationFn(ctx, rawToken)
	}
	return nil
}

func (s *stubAccountService) ResendVerification(ctx context.Context, accountID int) error {
	if s.resendVerificationFn != nil {
		return s.resendVerificationFn(ctx, accountID)
	}
	return nil
}

func (s *stubAccountService) RequestPasswordReset(ctx context.Context, email string) error {
	if s.requestResetFn != nil {
		return s.requestResetFn(ctx, email)
	}
	return nil
}

func (s *stubAccountService) ConsumePasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if s.consumeResetFn != nil {
		return s.consumeResetFn(ctx, rawToken, newPassword)
	}
	return nil
}

func (s *stubAccountService) Peek(ctx context.Context, kind actiontokendomain.Action, rawToken string) (*actiontokendomain.ActionToken, error) {
	if s.peekFn != nil {
		return s.peekFn(ctx, kind, rawToken)
	}
	return &actiontokendomain.ActionToken{}, nil
}

func (s *stubAccountService) UpdatePassword(ctx context.Context, userID int, currentRawToken, currentPassword, newPassword, confirmPassword string) error {
	s.updatePasswordCalls = append(s.updatePasswordCalls, updatePasswordCall{
		CurrentRawToken: currentRawToken,
		CurrentPassword: currentPassword,
		NewPassword:     newPassword,
		ConfirmPassword: confirmPassword,
		UserID:          userID,
	})
	if s.updatePasswordFn != nil {
		return s.updatePasswordFn(ctx, userID, currentRawToken, currentPassword, newPassword, confirmPassword)
	}
	return nil
}

func (s *stubAccountService) RequestEmailChange(ctx context.Context, userID int, currentPassword, newEmail string) error {
	s.requestEmailCalls = append(s.requestEmailCalls, requestEmailCall{
		CurrentPassword: currentPassword,
		NewEmail:        newEmail,
		UserID:          userID,
	})
	if s.requestEmailChangeFn != nil {
		return s.requestEmailChangeFn(ctx, userID, currentPassword, newEmail)
	}
	return nil
}

func (s *stubAccountService) ConsumeEmailChange(ctx context.Context, rawToken string) (*domain.User, error) {
	if s.consumeEmailChangeFn != nil {
		return s.consumeEmailChangeFn(ctx, rawToken)
	}
	return &domain.User{Email: "new@example.com"}, nil
}

type stubSessionService struct {
	createFn     func(context.Context, int) (string, *domain.Session, error)
	validateFn   func(context.Context, string) (*domain.Session, error)
	destroyFn    func(context.Context, string) error
	createCalls  []int
	destroyCalls []string
}

func (s *stubSessionService) Create(ctx context.Context, userID int) (string, *domain.Session, error) {
	s.createCalls = append(s.createCalls, userID)
	if s.createFn != nil {
		return s.createFn(ctx, userID)
	}
	return "stub-token", &domain.Session{UserID: userID}, nil
}

func (s *stubSessionService) Validate(ctx context.Context, rawToken string) (*domain.Session, error) {
	if s.validateFn != nil {
		return s.validateFn(ctx, rawToken)
	}
	return nil, domain.ErrSessionNotFound
}

func (s *stubSessionService) Destroy(ctx context.Context, rawToken string) error {
	s.destroyCalls = append(s.destroyCalls, rawToken)
	if s.destroyFn != nil {
		return s.destroyFn(ctx, rawToken)
	}
	return nil
}

func (s *stubSessionService) TTL() time.Duration { return stubSessionTTL }

type stubUserLookup struct {
	getByIDFn func(context.Context, int) (*domain.User, error)
}

func (s *stubUserLookup) GetByID(ctx context.Context, id int) (*domain.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &domain.User{ID: id}, nil
}

type stubCharacterLister struct {
	listFn func(context.Context, int) ([]characterapp.CharacterDTO, error)
}

func (s *stubCharacterLister) List(ctx context.Context, accountID int) ([]characterapp.CharacterDTO, error) {
	if s.listFn != nil {
		return s.listFn(ctx, accountID)
	}
	return nil, nil
}
