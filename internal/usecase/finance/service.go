// Package finance is the business logic for a center's manual money ledger: invoices (what a
// student owes) and payments (what reception recorded as paid). No payment gateway — payments are
// entered by hand. Balance/debt is derived. Management is a center_admin concern.
package finance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid input (non-positive amount or unknown payment method).
var ErrValidation = errors.New("finance: validation failed")

type Service struct {
	repo repo.FinanceRepository
}

func NewService(r repo.FinanceRepository) *Service { return &Service{repo: r} }

func (s *Service) ListInvoices(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Invoice, error) {
	return s.repo.ListInvoices(ctx, orgID, studentID)
}

func (s *Service) ListPayments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Payment, error) {
	return s.repo.ListPayments(ctx, orgID, studentID)
}

func (s *Service) StudentBalance(ctx context.Context, orgID, studentID uuid.UUID) (int64, error) {
	return s.repo.StudentBalance(ctx, orgID, studentID)
}

type InvoiceInput struct {
	EnrollmentID *uuid.UUID
	Amount       int64
	DueDate      *time.Time
	Period       string
	Note         string
}

func (s *Service) CreateInvoice(ctx context.Context, orgID, studentID uuid.UUID, in InvoiceInput) (entity.Invoice, error) {
	if in.Amount <= 0 {
		return entity.Invoice{}, ErrValidation
	}
	return s.repo.CreateInvoice(ctx, orgID, repo.CreateInvoiceParams{
		StudentID:    studentID,
		EnrollmentID: in.EnrollmentID,
		Amount:       in.Amount,
		DueDate:      in.DueDate,
		Period:       in.Period,
		Note:         in.Note,
	})
}

func (s *Service) DeleteInvoice(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteInvoice(ctx, orgID, id)
}

type PaymentInput struct {
	Amount int64
	Method string
	PaidAt *time.Time
	Note   string
}

func (s *Service) RecordPayment(ctx context.Context, orgID, invoiceID uuid.UUID, in PaymentInput) (entity.Payment, error) {
	if in.Amount <= 0 {
		return entity.Payment{}, ErrValidation
	}
	if in.Method == "" {
		in.Method = "cash"
	}
	if !entity.PaymentMethods[in.Method] {
		return entity.Payment{}, ErrValidation
	}
	return s.repo.RecordPayment(ctx, orgID, repo.RecordPaymentParams{
		InvoiceID: invoiceID,
		Amount:    in.Amount,
		Method:    in.Method,
		PaidAt:    in.PaidAt,
		Note:      in.Note,
	})
}

func (s *Service) DeletePayment(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeletePayment(ctx, orgID, id)
}

// RefundPayment records a refund against an invoice as a NEGATIVE payment: paid_amount drops (floored
// at 0), the student's balance rises, and income reflects the return. Reuses the payment ledger.
func (s *Service) RefundPayment(ctx context.Context, orgID, invoiceID uuid.UUID, in PaymentInput) (entity.Payment, error) {
	if in.Amount <= 0 {
		return entity.Payment{}, ErrValidation
	}
	if in.Method == "" {
		in.Method = "cash"
	}
	if !entity.PaymentMethods[in.Method] {
		return entity.Payment{}, ErrValidation
	}
	return s.repo.RecordPayment(ctx, orgID, repo.RecordPaymentParams{
		InvoiceID: invoiceID,
		Amount:    -in.Amount, // negative → refund
		Method:    in.Method,
		PaidAt:    in.PaidAt,
		Note:      in.Note,
	})
}

func (s *Service) Summary(ctx context.Context, orgID uuid.UUID) (entity.FinanceSummary, error) {
	return s.repo.Summary(ctx, orgID)
}

// GroupFinance is the group roster's billing state for a month ("2026-07").
func (s *Service) GroupFinance(ctx context.Context, orgID, groupID uuid.UUID, period string) ([]entity.GroupFinanceRow, error) {
	return s.repo.GroupFinance(ctx, orgID, groupID, period)
}

func (s *Service) Debtors(ctx context.Context, orgID uuid.UUID) ([]entity.Debtor, error) {
	return s.repo.ListDebtors(ctx, orgID)
}

type ExpenseInput struct {
	Category string
	Amount   int64
	SpentAt  *time.Time
	Note     string
}

func (s *Service) CreateExpense(ctx context.Context, orgID uuid.UUID, in ExpenseInput) (entity.Expense, error) {
	if in.Amount <= 0 {
		return entity.Expense{}, ErrValidation
	}
	if in.Category == "" {
		in.Category = "boshqa"
	}
	if in.SpentAt == nil {
		now := time.Now()
		in.SpentAt = &now
	}
	return s.repo.CreateExpense(ctx, orgID, repo.CreateExpenseParams{
		Category: in.Category,
		Amount:   in.Amount,
		SpentAt:  in.SpentAt,
		Note:     in.Note,
	})
}

// ListExpenses returns expenses in [from, to]; a zero range defaults to the current month.
func (s *Service) ListExpenses(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Expense, error) {
	from, to = defaultMonthRange(from, to)
	return s.repo.ListExpenses(ctx, orgID, from, to)
}

func (s *Service) DeleteExpense(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteExpense(ctx, orgID, id)
}

func (s *Service) ExpensesByCategory(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.CategoryTotal, error) {
	from, to = defaultMonthRange(from, to)
	return s.repo.ExpensesByCategory(ctx, orgID, from, to)
}

// defaultMonthRange falls back to the current calendar month when either bound is zero.
func defaultMonthRange(from, to time.Time) (time.Time, time.Time) {
	if from.IsZero() || to.IsZero() {
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = from.AddDate(0, 1, -1)
	}
	return from, to
}
