package entity

import (
	"time"

	"github.com/google/uuid"
)

// Invoice is what a student owes for a period (manual ledger — no payment gateway). paid_amount is
// the running sum of recorded payments; the status is derived (unpaid/partial/paid/overdue).
type Invoice struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	StudentID    uuid.UUID
	GroupID      *uuid.UUID // which group (course) the charge is for
	EnrollmentID *uuid.UUID
	Amount       int64 // total owed, whole UZS so'm
	PaidAmount   int64
	DueDate      *time.Time
	Period       string // e.g. "2026-07" or "Iyul"
	Note         string
	CreatedAt    time.Time
}

// Balance is the still-owed amount (never negative).
func (i Invoice) Balance() int64 {
	if i.PaidAmount >= i.Amount {
		return 0
	}
	return i.Amount - i.PaidAmount
}

// Status derives the invoice state from paid_amount and the due date.
func (i Invoice) Status(now time.Time) string {
	if i.PaidAmount >= i.Amount {
		return "paid"
	}
	if i.PaidAmount > 0 {
		return "partial"
	}
	if i.DueDate != nil && i.DueDate.Before(now) {
		return "overdue"
	}
	return "unpaid"
}

// Payment is a recorded payment against an invoice (cash/card/transfer, entered by reception).
type Payment struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	InvoiceID uuid.UUID
	StudentID uuid.UUID
	Amount    int64
	Method    string // cash | card | transfer
	PaidAt    time.Time
	Note      string
	CreatedAt time.Time
}

// PaymentMethods is the allowed set.
var PaymentMethods = map[string]bool{"cash": true, "card": true, "transfer": true}

// Expense is an operating cost (rent, utilities, salary, ads, ...). profit = income − expenses.
type Expense struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Category  string
	Amount    int64
	SpentAt   time.Time
	Note      string
	BranchID  *uuid.UUID
	CreatedAt time.Time
}

// FinanceSummary is the center's money snapshot for the dashboard.
type FinanceSummary struct {
	TodayIncome  int64
	MonthIncome  int64
	TotalDebt    int64
	TodayExpense int64
	MonthExpense int64
}

// MonthProfit is this month's income minus this month's expenses.
func (s FinanceSummary) MonthProfit() int64 { return s.MonthIncome - s.MonthExpense }

// CategoryTotal is an expense sum grouped by category (for reports).
type CategoryTotal struct {
	Category string
	Total    int64
}

// Debtor is a student with an outstanding balance.
type Debtor struct {
	StudentID uuid.UUID
	Name      string
	Balance   int64
}

// GroupFinanceRow is one student's billing state for a period (month) inside a group roster.
type GroupFinanceRow struct {
	StudentID uuid.UUID
	Name      string
	Invoiced  int64
	Paid      int64
	Attended  int64 // sessions attended in this group this period (grace-period signal)
}

// OverdueInvoice is an unpaid invoice past its due date (payment reminder).
type OverdueInvoice struct {
	Invoice
	StudentName string
}

// GraceOverdue flags a group member attending sessions without an invoice this period.
type GraceOverdue struct {
	StudentID   uuid.UUID
	StudentName string
	GroupID     uuid.UUID
	GroupName   string
	Attended    int64
}
