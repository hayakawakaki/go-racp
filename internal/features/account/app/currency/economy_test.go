package currency

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hayakawakaki/go-racp/internal/features/account/domain"
)

func TestService_Totals_Maps(t *testing.T) {
	t.Parallel()

	repo := &fakeCurrencyRepo{
		totalsFn: func(context.Context) (domain.CurrencyTotals, error) {
			return domain.CurrencyTotals{Zeny: 9000, Cashpoint: 4200}, nil
		},
	}
	svc := NewService(repo)

	got, err := svc.Totals(context.Background())
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}
	if got.Zeny != 9000 || got.Cashpoint != 4200 {
		t.Errorf("Totals = %+v, want {Zeny:9000 Cashpoint:4200}", got)
	}
}

func TestService_DepositHistory_OffsetAndPaging(t *testing.T) {
	t.Parallel()

	var gotLimit, gotOffset int
	repo := &fakeCurrencyRepo{
		listDepositsFn: func(_ context.Context, limit, offset int) ([]domain.DepositRecord, int, error) {
			gotLimit = limit
			gotOffset = offset
			return []domain.DepositRecord{{DepositID: 5, AccountID: 1, Zeny: 100, Cashpoint: 10, ProcessedAt: time.Unix(0, 0)}}, 31, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.DepositHistory(context.Background(), 3, 15)
	if err != nil {
		t.Fatalf("DepositHistory: %v", err)
	}
	if gotLimit != 15 || gotOffset != 30 {
		t.Errorf("repo called with limit=%d offset=%d, want 15/30", gotLimit, gotOffset)
	}
	if page.Total != 31 || page.Page != 3 || page.PerPage != 15 || page.TotalPages != 3 {
		t.Errorf("page meta = %+v, want Total:31 Page:3 PerPage:15 TotalPages:3", page)
	}
	if len(page.Rows) != 1 || page.Rows[0].DepositID != 5 {
		t.Errorf("rows = %+v, want one row id 5", page.Rows)
	}
}

func TestService_DepositHistory_ClampsPage(t *testing.T) {
	t.Parallel()

	var gotOffset int
	repo := &fakeCurrencyRepo{
		listDepositsFn: func(_ context.Context, _, offset int) ([]domain.DepositRecord, int, error) {
			gotOffset = offset
			return nil, 0, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.DepositHistory(context.Background(), 0, 15)
	if err != nil {
		t.Fatalf("DepositHistory: %v", err)
	}
	if gotOffset != 0 || page.Page != 1 {
		t.Errorf("page<1 should clamp to 1 with offset 0, got offset=%d page=%d", gotOffset, page.Page)
	}
}

func TestService_WithdrawHistory_Maps(t *testing.T) {
	t.Parallel()

	sentAt := time.Unix(100, 0)
	repo := &fakeCurrencyRepo{
		listWithdrawsFn: func(context.Context, int, int) ([]domain.WithdrawRecord, int, error) {
			return []domain.WithdrawRecord{
				{ID: 2, AccountID: 1, Zeny: 200, Cashpoint: 20, Status: 2, CreatedAt: time.Unix(0, 0), SentAt: &sentAt},
				{ID: 1, AccountID: 1, Zeny: 100, Cashpoint: 10, Status: 1, CreatedAt: time.Unix(0, 0)},
			}, 2, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.WithdrawHistory(context.Background(), 1, 15)
	if err != nil {
		t.Fatalf("WithdrawHistory: %v", err)
	}
	if len(page.Rows) != 2 || page.Rows[0].ID != 2 || page.Rows[0].Status != 2 || page.Rows[0].SentAt == nil {
		t.Errorf("rows = %+v, want sent row first", page.Rows)
	}
	if page.Rows[1].SentAt != nil {
		t.Errorf("pending row SentAt = %v, want nil", page.Rows[1].SentAt)
	}
}

func TestService_DepositHistoryByAccount_OffsetAndPaging(t *testing.T) {
	t.Parallel()

	var gotAccountID, gotLimit, gotOffset int
	repo := &fakeCurrencyRepo{
		listDepositsByAccountFn: func(_ context.Context, accountID, limit, offset int) ([]domain.DepositRecord, int, error) {
			gotAccountID = accountID
			gotLimit = limit
			gotOffset = offset
			return []domain.DepositRecord{{DepositID: 5, AccountID: accountID, Zeny: 100, Cashpoint: 10, ProcessedAt: time.Unix(0, 0)}}, 31, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.DepositHistoryByAccount(context.Background(), 7, 3, 15)
	if err != nil {
		t.Fatalf("DepositHistoryByAccount: %v", err)
	}
	if gotAccountID != 7 || gotLimit != 15 || gotOffset != 30 {
		t.Errorf("repo called with account=%d limit=%d offset=%d, want 7/15/30", gotAccountID, gotLimit, gotOffset)
	}
	if page.Total != 31 || page.Page != 3 || page.PerPage != 15 || page.TotalPages != 3 {
		t.Errorf("page meta = %+v, want Total:31 Page:3 PerPage:15 TotalPages:3", page)
	}
	if len(page.Rows) != 1 || page.Rows[0].DepositID != 5 {
		t.Errorf("rows = %+v, want one row id 5", page.Rows)
	}
}

func TestService_WithdrawHistoryByAccount_Maps(t *testing.T) {
	t.Parallel()

	var gotAccountID int
	sentAt := time.Unix(100, 0)
	repo := &fakeCurrencyRepo{
		listWithdrawsByAccountFn: func(_ context.Context, accountID, _, _ int) ([]domain.WithdrawRecord, int, error) {
			gotAccountID = accountID
			return []domain.WithdrawRecord{
				{ID: 2, AccountID: accountID, Zeny: 200, Cashpoint: 20, Status: 2, CreatedAt: time.Unix(0, 0), SentAt: &sentAt},
				{ID: 1, AccountID: accountID, Zeny: 100, Cashpoint: 10, Status: 1, CreatedAt: time.Unix(0, 0)},
			}, 2, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.WithdrawHistoryByAccount(context.Background(), 9, 1, 15)
	if err != nil {
		t.Fatalf("WithdrawHistoryByAccount: %v", err)
	}
	if gotAccountID != 9 {
		t.Errorf("account = %d, want 9", gotAccountID)
	}
	if len(page.Rows) != 2 || page.Rows[0].ID != 2 || page.Rows[0].Status != 2 || page.Rows[0].SentAt == nil {
		t.Errorf("rows = %+v, want sent row first", page.Rows)
	}
	if page.Rows[1].SentAt != nil {
		t.Errorf("pending row SentAt = %v, want nil", page.Rows[1].SentAt)
	}
}

func TestService_StuckWithdraws_MapsWithThreshold(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)
	reapAfter := 30 * time.Minute

	var gotBefore time.Time
	var gotLimit int
	repo := &fakeCurrencyRepo{
		sentBeforeFn: func(_ context.Context, before time.Time, limit int) ([]domain.WithdrawRecord, error) {
			gotBefore = before
			gotLimit = limit
			return []domain.WithdrawRecord{
				{ID: 4, AccountID: 7, Zeny: 500, Cashpoint: 25, Status: 2, CreatedAt: time.Unix(0, 0)},
			}, nil
		},
	}
	svc := NewService(repo, WithReapAfter(reapAfter), WithNow(func() time.Time { return now }))

	rows, err := svc.StuckWithdraws(context.Background())
	if err != nil {
		t.Fatalf("StuckWithdraws: %v", err)
	}
	if want := now.Add(-reapAfter); !gotBefore.Equal(want) {
		t.Errorf("SentBefore threshold = %v, want %v", gotBefore, want)
	}
	if gotLimit != stuckWithdrawLimit {
		t.Errorf("SentBefore limit = %d, want %d", gotLimit, stuckWithdrawLimit)
	}
	if len(rows) != 1 || rows[0].ID != 4 || rows[0].AccountID != 7 || rows[0].Status != 2 {
		t.Errorf("rows = %+v, want one stuck row id 4", rows)
	}
}

func TestService_StuckWithdraws_DisabledWhenReapAfterZero(t *testing.T) {
	t.Parallel()

	called := false
	repo := &fakeCurrencyRepo{
		sentBeforeFn: func(context.Context, time.Time, int) ([]domain.WithdrawRecord, error) {
			called = true
			return nil, nil
		},
	}
	svc := NewService(repo)

	rows, err := svc.StuckWithdraws(context.Background())
	if err != nil {
		t.Fatalf("StuckWithdraws: %v", err)
	}
	if rows != nil {
		t.Errorf("rows = %+v, want nil when ReapAfter is unset", rows)
	}
	if called {
		t.Errorf("SentBefore must not be called when ReapAfter is unset")
	}
}

func TestService_StuckWithdraws_WrapsRepoError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("sent before db down")
	repo := &fakeCurrencyRepo{
		sentBeforeFn: func(context.Context, time.Time, int) ([]domain.WithdrawRecord, error) {
			return nil, wantErr
		},
	}
	svc := NewService(repo, WithReapAfter(30*time.Minute))

	_, err := svc.StuckWithdraws(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want wantErr in chain", err)
	}
}

func TestService_WithdrawHistory_MapsDeliveredAt(t *testing.T) {
	t.Parallel()

	sentAt := time.Unix(100, 0)
	deliveredAt := time.Unix(200, 0)
	repo := &fakeCurrencyRepo{
		listWithdrawsFn: func(context.Context, int, int) ([]domain.WithdrawRecord, int, error) {
			return []domain.WithdrawRecord{
				{ID: 3, AccountID: 1, Zeny: 300, Cashpoint: 30, Status: 3, CreatedAt: time.Unix(0, 0), SentAt: &sentAt, DeliveredAt: &deliveredAt},
			}, 1, nil
		},
	}
	svc := NewService(repo)

	page, err := svc.WithdrawHistory(context.Background(), 1, 15)
	if err != nil {
		t.Fatalf("WithdrawHistory: %v", err)
	}
	if len(page.Rows) != 1 || page.Rows[0].DeliveredAt == nil || !page.Rows[0].DeliveredAt.Equal(deliveredAt) {
		t.Errorf("DeliveredAt = %v, want %v", page.Rows[0].DeliveredAt, deliveredAt)
	}
}
