package state

import (
	currency "github.com/hayakawakaki/go-racp/internal/features/account/app/currency"
	app "github.com/hayakawakaki/go-racp/internal/features/account/app/self"
	charapp "github.com/hayakawakaki/go-racp/internal/features/character/app"
)

type AccountState struct {
	Account         *app.AccountDTO
	Notice          string
	BanBlocked      string
	Characters      []charapp.CharacterDTO
	RecentWithdraws []currency.WithdrawDTO
	Balance         currency.BalanceDTO
}

type ChangeEmailState struct {
	Errors   map[string]string
	NewEmail string
}

type ChangePasswordState struct {
	Errors map[string]string
}

type EmailChangeResultKind int

const (
	EmailChangeResultSuccess EmailChangeResultKind = iota
	EmailChangeResultExpired
	EmailChangeResultInvalid
	EmailChangeResultAlready
	EmailChangeResultTaken
	EmailChangeResultAccountRestricted
)

type EmailChangeResultState struct {
	NewEmail string
	Kind     EmailChangeResultKind
}

type ForgotPasswordState struct {
	Errors    map[string]string
	Email     string
	Submitted bool
}

type LoginFormState struct {
	Username string
	Error    string
	Notice   string
}

type RegisterFormState struct {
	Username     string
	Email        string
	Gender       string
	Birthdate    string
	BirthdateMin string
	BirthdateMax string
	Errors       map[string]string
	FormError    string
}

type ResetPasswordState struct {
	Errors map[string]string
	Token  string
}

type ResetResultKind int

const (
	ResetResultSuccess ResetResultKind = iota
	ResetResultExpired
	ResetResultInvalid
	ResetResultAlreadyUsed
	ResetResultAccountRestricted
)

type ResetResultState struct {
	Kind ResetResultKind
}

type VerifyAccountState struct {
	Email  string
	Notice string
}

type VerifyConfirmState struct {
	Token string
}

type VerifyEmailChangeConfirmState struct {
	Token    string
	NewEmail string
}

type VerifyResultKind int

const (
	VerifyResultSuccess VerifyResultKind = iota
	VerifyResultExpired
	VerifyResultInvalid
)

type VerifyResultState struct {
	Kind VerifyResultKind
}
