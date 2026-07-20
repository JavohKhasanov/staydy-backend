package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// FinanceRepository persists invoices + payments. Both are RLS-protected, so every call runs
// inside the tenant scope. Payment writes also adjust the invoice's denormalised paid_amount, so
// they run in a single transaction (WithTenant gives one).
type FinanceRepository struct {
	db *postgres.DB
}

func NewFinanceRepository(db *postgres.DB) *FinanceRepository {
	return &FinanceRepository{db: db}
}

func (r *FinanceRepository) ListInvoices(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Invoice, error) {
	var out []entity.Invoice
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListInvoicesByStudent(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.Invoice, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapInvoice(row))
		}
		return nil
	})
	return out, err
}

func (r *FinanceRepository) CreateInvoice(ctx context.Context, orgID uuid.UUID, p repo.CreateInvoiceParams) (entity.Invoice, error) {
	var row sqlc.Invoice
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateInvoice(ctx, sqlc.CreateInvoiceParams{
			OrgID:        orgID,
			StudentID:    p.StudentID,
			GroupID:      nullableUUID(p.GroupID),
			EnrollmentID: nullableUUID(p.EnrollmentID),
			Amount:       p.Amount,
			DueDate:      dateVal(p.DueDate),
			Period:       p.Period,
			Note:         p.Note,
		})
		return e
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.Invoice{}, repo.ErrNotFound
		}
		return entity.Invoice{}, err
	}
	return mapInvoice(row), nil
}

func (r *FinanceRepository) DeleteInvoice(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteInvoice(ctx, sqlc.DeleteInvoiceParams{OrgID: orgID, ID: id})
	})
}

func (r *FinanceRepository) StudentBalance(ctx context.Context, orgID, studentID uuid.UUID) (int64, error) {
	var bal int64
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		bal, e = sqlc.New(tx).StudentBalance(ctx, studentID)
		return e
	})
	return bal, err
}

func (r *FinanceRepository) ListPayments(ctx context.Context, orgID, studentID uuid.UUID) ([]entity.Payment, error) {
	var out []entity.Payment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListPaymentsByStudent(ctx, studentID)
		if e != nil {
			return e
		}
		out = make([]entity.Payment, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapPayment(row))
		}
		return nil
	})
	return out, err
}

func (r *FinanceRepository) RecordPayment(ctx context.Context, orgID uuid.UUID, p repo.RecordPaymentParams) (entity.Payment, error) {
	var pay sqlc.Payment
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		inv, e := q.GetInvoice(ctx, sqlc.GetInvoiceParams{OrgID: orgID, ID: p.InvoiceID})
		if e != nil {
			return e // pgx.ErrNoRows → invoice not found
		}
		// Positive payments may not exceed the remaining balance (refunds are negative and skip
		// this; prepayment/credit is a separate future feature).
		if p.Amount > 0 && p.Amount > inv.Amount-inv.PaidAmount {
			return repo.ErrOverpay
		}
		paidAt := time.Now()
		if p.PaidAt != nil {
			paidAt = *p.PaidAt
		}
		pay, e = q.CreatePayment(ctx, sqlc.CreatePaymentParams{
			OrgID:     orgID,
			InvoiceID: p.InvoiceID,
			StudentID: inv.StudentID,
			Amount:    p.Amount,
			Method:    p.Method,
			PaidAt:    tsVal(paidAt),
			Note:      p.Note,
		})
		if e != nil {
			return e
		}
		return q.AdjustInvoicePaid(ctx, sqlc.AdjustInvoicePaidParams{OrgID: orgID, ID: p.InvoiceID, PaidAmount: p.Amount})
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Payment{}, repo.ErrNotFound
		}
		return entity.Payment{}, err
	}
	return mapPayment(pay), nil
}

func (r *FinanceRepository) DeletePayment(ctx context.Context, orgID, id uuid.UUID) error {
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		pay, e := q.GetPayment(ctx, sqlc.GetPaymentParams{OrgID: orgID, ID: id})
		if e != nil {
			return e
		}
		if e := q.DeletePayment(ctx, sqlc.DeletePaymentParams{OrgID: orgID, ID: id}); e != nil {
			return e
		}
		// Reverse the payment from the invoice (AdjustInvoicePaid floors at 0).
		return q.AdjustInvoicePaid(ctx, sqlc.AdjustInvoicePaidParams{OrgID: orgID, ID: pay.InvoiceID, PaidAmount: -pay.Amount})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return repo.ErrNotFound
	}
	return err
}

func (r *FinanceRepository) Summary(ctx context.Context, orgID uuid.UUID) (entity.FinanceSummary, error) {
	var s entity.FinanceSummary
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).FinanceSummary(ctx, orgID)
		if e != nil {
			return e
		}
		s = entity.FinanceSummary{
			TodayIncome:  row.TodayIncome,
			MonthIncome:  row.MonthIncome,
			TotalDebt:    row.TotalDebt,
			TodayExpense: row.TodayExpense,
			MonthExpense: row.MonthExpense,
		}
		return nil
	})
	return s, err
}

func (r *FinanceRepository) CreateExpense(ctx context.Context, orgID uuid.UUID, p repo.CreateExpenseParams) (entity.Expense, error) {
	var row sqlc.Expense
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateExpense(ctx, sqlc.CreateExpenseParams{
			OrgID:    orgID,
			Category: p.Category,
			Amount:   p.Amount,
			SpentAt:  dateVal(p.SpentAt),
			Note:     p.Note,
		})
		return e
	})
	if err != nil {
		return entity.Expense{}, err
	}
	return mapExpense(row), nil
}

func (r *FinanceRepository) ListExpenses(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Expense, error) {
	var out []entity.Expense
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListExpenses(ctx, sqlc.ListExpensesParams{
			OrgID:     orgID,
			SpentAt:   dateVal(&from),
			SpentAt_2: dateVal(&to),
		})
		if e != nil {
			return e
		}
		out = make([]entity.Expense, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapExpense(row))
		}
		return nil
	})
	return out, err
}

func (r *FinanceRepository) DeleteExpense(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteExpense(ctx, id)
	})
}

func (r *FinanceRepository) ExpensesByCategory(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.CategoryTotal, error) {
	var out []entity.CategoryTotal
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ExpensesByCategory(ctx, sqlc.ExpensesByCategoryParams{
			OrgID:     orgID,
			SpentAt:   dateVal(&from),
			SpentAt_2: dateVal(&to),
		})
		if e != nil {
			return e
		}
		out = make([]entity.CategoryTotal, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.CategoryTotal{Category: row.Category, Total: row.Total})
		}
		return nil
	})
	return out, err
}

func mapExpense(e sqlc.Expense) entity.Expense {
	return entity.Expense{
		ID:        e.ID,
		OrgID:     e.OrgID,
		Category:  e.Category,
		Amount:    e.Amount,
		SpentAt:   e.SpentAt.Time,
		Note:      e.Note,
		BranchID:  uuidToPtr(e.BranchID),
		CreatedAt: e.CreatedAt.Time,
	}
}

func (r *FinanceRepository) ListDebtors(ctx context.Context, orgID uuid.UUID) ([]entity.Debtor, error) {
	var out []entity.Debtor
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListDebtors(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.Debtor, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.Debtor{StudentID: row.StudentID, Name: row.Name, Balance: row.Balance})
		}
		return nil
	})
	return out, err
}

func (r *FinanceRepository) GroupFinance(ctx context.Context, orgID, groupID uuid.UUID, period string) ([]entity.GroupFinanceRow, error) {
	var out []entity.GroupFinanceRow
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).GroupFinanceByPeriod(ctx, sqlc.GroupFinanceByPeriodParams{
			Period:  period,
			GroupID: groupID,
		})
		if e != nil {
			return e
		}
		out = make([]entity.GroupFinanceRow, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.GroupFinanceRow{
				StudentID: row.StudentID,
				Name:      row.StudentName,
				Invoiced:  row.Invoiced,
				Paid:      row.Paid,
				Attended:  row.Attended,
			})
		}
		return nil
	})
	return out, err
}

func mapInvoice(i sqlc.Invoice) entity.Invoice {
	return entity.Invoice{
		ID:           i.ID,
		OrgID:        i.OrgID,
		StudentID:    i.StudentID,
		GroupID:      uuidToPtr(i.GroupID),
		EnrollmentID: uuidToPtr(i.EnrollmentID),
		Amount:       i.Amount,
		PaidAmount:   i.PaidAmount,
		DueDate:      dateToPtr(i.DueDate),
		Period:       i.Period,
		Note:         i.Note,
		CreatedAt:    i.CreatedAt.Time,
	}
}

func mapPayment(p sqlc.Payment) entity.Payment {
	return entity.Payment{
		ID:        p.ID,
		OrgID:     p.OrgID,
		InvoiceID: p.InvoiceID,
		StudentID: p.StudentID,
		Amount:    p.Amount,
		Method:    p.Method,
		PaidAt:    p.PaidAt.Time,
		Note:      p.Note,
		CreatedAt: p.CreatedAt.Time,
	}
}

func (r *FinanceRepository) OverdueInvoices(ctx context.Context, orgID uuid.UUID) ([]entity.OverdueInvoice, error) {
	var out []entity.OverdueInvoice
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).OverdueInvoices(ctx)
		if e != nil {
			return e
		}
		out = make([]entity.OverdueInvoice, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.OverdueInvoice{
				Invoice: mapInvoice(sqlc.Invoice{
					ID: row.ID, OrgID: row.OrgID, StudentID: row.StudentID, GroupID: row.GroupID,
					EnrollmentID: row.EnrollmentID, Amount: row.Amount, PaidAmount: row.PaidAmount,
					DueDate: row.DueDate, Period: row.Period, Note: row.Note, CreatedAt: row.CreatedAt,
				}),
				StudentName: row.StudentName,
			})
		}
		return nil
	})
	return out, err
}

func (r *FinanceRepository) GraceOverdue(ctx context.Context, orgID uuid.UUID, period string) ([]entity.GraceOverdue, error) {
	var out []entity.GraceOverdue
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).GraceOverdueStudents(ctx, period)
		if e != nil {
			return e
		}
		out = make([]entity.GraceOverdue, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.GraceOverdue{
				StudentID:   row.StudentID,
				StudentName: row.StudentName,
				GroupID:     row.GroupID,
				GroupName:   row.GroupName,
				Attended:    row.Attended,
			})
		}
		return nil
	})
	return out, err
}

// Grace setting lives on organizations (non-RLS) — read/write via the pool directly.
func (r *FinanceRepository) GetGraceLessons(ctx context.Context, orgID uuid.UUID) (int, error) {
	n, err := sqlc.New(r.db.Pool).GetGraceLessons(ctx, orgID)
	return int(n), err
}

func (r *FinanceRepository) SetGraceLessons(ctx context.Context, orgID uuid.UUID, n int) error {
	return sqlc.New(r.db.Pool).SetGraceLessons(ctx, sqlc.SetGraceLessonsParams{ID: orgID, GraceLessons: int32(n)})
}
