package currency

import (
	"context"
	"fmt"
	"time"
)

type TotalsDTO struct {
	Zeny      int64
	Cashpoint int64
}

type DepositDTO struct {
	ProcessedAt time.Time
	DepositID   int64
	AccountID   int
	Zeny        int64
	Cashpoint   int
}

type AdminWithdrawDTO struct {
	CreatedAt time.Time
	SentAt    *time.Time
	ID        int64
	AccountID int
	Zeny      int64
	Cashpoint int
	Status    int
}

type DepositPage struct {
	Rows       []DepositDTO
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

type WithdrawHistoryPage struct {
	Rows       []AdminWithdrawDTO
	Total      int
	Page       int
	PerPage    int
	TotalPages int
}

func (s *Service) Totals(ctx context.Context) (TotalsDTO, error) {
	totals, err := s.repo.Totals(ctx)
	if err != nil {
		return TotalsDTO{}, fmt.Errorf("currency.Service.Totals: %w", err)
	}

	return TotalsDTO{Zeny: totals.Zeny, Cashpoint: totals.Cashpoint}, nil
}

func (s *Service) DepositHistory(ctx context.Context, page, perPage int) (DepositPage, error) {
	page = normalizePage(page)
	offset := (page - 1) * perPage

	records, total, err := s.repo.ListDeposits(ctx, perPage, offset)
	if err != nil {
		return DepositPage{}, fmt.Errorf("currency.Service.DepositHistory: %w", err)
	}

	rows := make([]DepositDTO, 0, len(records))
	for _, record := range records {
		rows = append(rows, DepositDTO{
			DepositID:   record.DepositID,
			AccountID:   record.AccountID,
			Zeny:        record.Zeny,
			Cashpoint:   record.Cashpoint,
			ProcessedAt: record.ProcessedAt,
		})
	}

	return DepositPage{
		Rows:       rows,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: pageCount(total, perPage),
	}, nil
}

func (s *Service) WithdrawHistory(ctx context.Context, page, perPage int) (WithdrawHistoryPage, error) {
	page = normalizePage(page)
	offset := (page - 1) * perPage

	records, total, err := s.repo.ListWithdraws(ctx, perPage, offset)
	if err != nil {
		return WithdrawHistoryPage{}, fmt.Errorf("currency.Service.WithdrawHistory: %w", err)
	}

	rows := make([]AdminWithdrawDTO, 0, len(records))
	for _, record := range records {
		rows = append(rows, AdminWithdrawDTO{
			ID:        record.ID,
			AccountID: record.AccountID,
			Zeny:      record.Zeny,
			Cashpoint: record.Cashpoint,
			Status:    record.Status,
			CreatedAt: record.CreatedAt,
			SentAt:    record.SentAt,
		})
	}

	return WithdrawHistoryPage{
		Rows:       rows,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: pageCount(total, perPage),
	}, nil
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}

	return page
}

func pageCount(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}

	return (total + perPage - 1) / perPage
}
