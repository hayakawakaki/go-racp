package infra

import (
	"errors"
	"math"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/market/domain"
)

func TestAmountsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		zeny      int64
		cashpoint int
		want      bool
	}{
		{name: "valid", zeny: 1000, cashpoint: 10, want: true},
		{name: "zero", zeny: 0, cashpoint: 0, want: true},
		{name: "negative zeny", zeny: -1, cashpoint: 0, want: false},
		{name: "negative cashpoint", zeny: 0, cashpoint: -1, want: false},
		{name: "zeny at cap", zeny: domain.MaxTransferZeny, cashpoint: 0, want: true},
		{name: "zeny over cap", zeny: domain.MaxTransferZeny + 1, cashpoint: 0, want: false},
		{name: "cashpoint at int32 max", zeny: 0, cashpoint: math.MaxInt32, want: true},
		{name: "cashpoint above int32 max", zeny: 0, cashpoint: math.MaxInt32 + 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := amountsValid(tt.zeny, tt.cashpoint); got != tt.want {
				t.Errorf("amountsValid(%d, %d) = %v, want %v", tt.zeny, tt.cashpoint, got, tt.want)
			}
		})
	}
}

func TestValidateCharge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr        error
		name           string
		payer          int
		payee          int
		payZeny        int64
		payCashpoint   int
		payeeZeny      int64
		payeeCashpoint int
	}{
		{name: "valid", payer: 1, payee: 2, payZeny: 1000, payeeZeny: 980, wantErr: nil},
		{name: "negative pay", payer: 1, payee: 2, payZeny: -1, wantErr: domain.ErrInvalidAmount},
		{name: "negative payee", payer: 1, payee: 2, payZeny: 1000, payeeZeny: -1, wantErr: domain.ErrInvalidAmount},
		{name: "self trade", payer: 1, payee: 1, payZeny: 1000, payeeZeny: 1000, wantErr: domain.ErrSelfTrade},
		{name: "payee exceeds pay zeny", payer: 1, payee: 2, payZeny: 1000, payeeZeny: 1001, wantErr: domain.ErrInvalidSettlement},
		{name: "payee exceeds pay cashpoint", payer: 1, payee: 2, payZeny: 1000, payCashpoint: 10, payeeZeny: 1000, payeeCashpoint: 11, wantErr: domain.ErrInvalidSettlement},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateCharge(tt.payer, tt.payee, tt.payZeny, tt.payCashpoint, tt.payeeZeny, tt.payeeCashpoint)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validateCharge() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePartialSettle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr        error
		name           string
		grossZeny      int64
		grossCashpoint int
		payeeZeny      int64
		payeeCashpoint int
	}{
		{name: "valid", grossZeny: 1000, payeeZeny: 980, wantErr: nil},
		{name: "negative gross", grossZeny: -1, wantErr: domain.ErrInvalidAmount},
		{name: "negative payee", grossZeny: 1000, payeeZeny: -1, wantErr: domain.ErrInvalidAmount},
		{name: "payee exceeds gross zeny", grossZeny: 1000, payeeZeny: 1001, wantErr: domain.ErrInvalidSettlement},
		{name: "payee exceeds gross cashpoint", grossZeny: 1000, grossCashpoint: 10, payeeZeny: 1000, payeeCashpoint: 11, wantErr: domain.ErrInvalidSettlement},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePartialSettle(tt.grossZeny, tt.grossCashpoint, tt.payeeZeny, tt.payeeCashpoint)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("validatePartialSettle() err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
