package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/repo"
	financeusecase "github.com/student-success/backend/internal/usecase/finance"
)

type invoiceResponse struct {
	ID         string  `json:"id"`
	StudentID  string  `json:"studentId"`
	GroupID    string  `json:"groupId,omitempty"`
	Amount     int64   `json:"amount"`
	PaidAmount int64   `json:"paidAmount"`
	Balance    int64   `json:"balance"`
	Status     string  `json:"status"`
	DueDate    *string `json:"dueDate,omitempty"`
	Period     string  `json:"period"`
	Note       string  `json:"note"`
	CreatedAt  string  `json:"createdAt"`
}

type paymentResponse struct {
	ID        string `json:"id"`
	InvoiceID string `json:"invoiceId"`
	StudentID string `json:"studentId"`
	Amount    int64  `json:"amount"`
	Method    string `json:"method"`
	Note      string `json:"note"`
	PaidAt    string `json:"paidAt"`
}

type studentFinanceResponse struct {
	Balance  int64             `json:"balance"`
	Invoices []invoiceResponse `json:"invoices"`
	Payments []paymentResponse `json:"payments"`
}

type studentFinanceInput struct {
	ID string `path:"id" format:"uuid"`
}
type studentFinanceOutput struct{ Body studentFinanceResponse }

type createInvoiceInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Amount   int64  `json:"amount" minimum:"1" doc:"whole UZS so'm"`
		GroupID  string `json:"groupId,omitempty" format:"uuid" doc:"which group (course) the charge is for"`
		DueDate  string `json:"dueDate,omitempty" doc:"YYYY-MM-DD"`
		Period   string `json:"period,omitempty" example:"2026-07"`
		Note     string `json:"note,omitempty" maxLength:"500"`
		Enrollment string `json:"enrollmentId,omitempty"`
	}
}
type invoiceOutput struct{ Body invoiceResponse }

type recordPaymentInput struct {
	ID   string `path:"id" format:"uuid"` // invoice id
	Body struct {
		Amount int64  `json:"amount" minimum:"1"`
		Method string `json:"method,omitempty" enum:"cash,card,transfer"`
		PaidAt string `json:"paidAt,omitempty" doc:"YYYY-MM-DD; defaults to now"`
		Note   string `json:"note,omitempty" maxLength:"500"`
	}
}
type paymentOutput struct{ Body paymentResponse }

type financeIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type deleteFinanceOutput struct{}

type generateInvoicesInput struct {
	Body struct {
		Period string `json:"period" doc:"YYYY-MM"`
	}
}
type generateInvoicesOutput struct {
	Body struct {
		Created int64 `json:"created"`
	}
}

type financeSummaryInput struct{}
type financeSummaryOutput struct {
	Body struct {
		TodayIncome  int64 `json:"todayIncome"`
		MonthIncome  int64 `json:"monthIncome"`
		TotalDebt    int64 `json:"totalDebt"`
		TodayExpense int64 `json:"todayExpense"`
		MonthExpense int64 `json:"monthExpense"`
		MonthProfit  int64 `json:"monthProfit"`
	}
}

type expenseResponse struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Amount   int64  `json:"amount"`
	SpentAt  string `json:"spentAt"`
	Note     string `json:"note"`
}
type listExpensesInput struct {
	From string `query:"from" doc:"YYYY-MM-DD (default: month start)"`
	To   string `query:"to" doc:"YYYY-MM-DD (default: month end)"`
}
type expenseCategoryTotal struct {
	Category string `json:"category"`
	Total    int64  `json:"total"`
}
type expensesListBody struct {
	Expenses   []expenseResponse      `json:"expenses"`
	Total      int64                  `json:"total"`
	ByCategory []expenseCategoryTotal `json:"byCategory"`
}
type expensesListOutput struct{ Body expensesListBody }
type createExpenseInput struct {
	Body struct {
		Category string `json:"category,omitempty" maxLength:"64"`
		Amount   int64  `json:"amount" minimum:"1" doc:"whole UZS so'm"`
		SpentAt  string `json:"spentAt,omitempty" doc:"YYYY-MM-DD; defaults to today"`
		Note     string `json:"note,omitempty" maxLength:"500"`
	}
}
type expenseOutput struct{ Body expenseResponse }
type debtorsOutput struct {
	Body []struct {
		StudentID string `json:"studentId"`
		Name      string `json:"name"`
		Balance   int64  `json:"balance"`
	}
}

type overdueInvoiceRow struct {
	StudentID   string  `json:"studentId"`
	StudentName string  `json:"studentName"`
	GroupID     string  `json:"groupId,omitempty"`
	Balance     int64   `json:"balance"`
	DueDate     *string `json:"dueDate,omitempty"`
	Period      string  `json:"period"`
}
type graceOverdueRow struct {
	StudentID   string `json:"studentId"`
	StudentName string `json:"studentName"`
	GroupID     string `json:"groupId"`
	GroupName   string `json:"groupName"`
	Attended    int64  `json:"attended"`
}
type paymentAlertsBody struct {
	OverdueInvoices []overdueInvoiceRow `json:"overdueInvoices"`
	GraceOverdue    []graceOverdueRow   `json:"graceOverdue"`
}
type paymentAlertsOutput struct{ Body paymentAlertsBody }

type billingSettingsBody struct {
	GraceLessons int `json:"graceLessons" minimum:"0" maximum:"30" doc:"attended sessions allowed before missing-invoice flag"`
}
type billingSettingsInput struct{ Body billingSettingsBody }
type billingSettingsOutput struct{ Body billingSettingsBody }

type groupFinanceRow struct {
	StudentID string `json:"studentId"`
	Name      string `json:"name"`
	Invoiced  int64  `json:"invoiced"`
	Paid      int64  `json:"paid"`
	Attended  int64  `json:"attended"`
}
type groupFinanceBody struct {
	Period   string            `json:"period"`
	Students []groupFinanceRow `json:"students"`
}
type groupFinanceInput struct {
	ID    string `path:"id" format:"uuid"`
	Month string `query:"month" doc:"YYYY-MM (default: current month)"`
}
type groupFinanceOutput struct{ Body groupFinanceBody }

// registerFinance mounts the money ledger across two gates. Operational endpoints (student
// balances, invoices, payments, debtors, group fee roster, alerts, billing settings) go on `api`,
// which admits the front-desk administrator (manager) so they can collect payments. The sensitive
// aggregate endpoints (center profit/expense summary, expense ledger) go on `sensitiveAPI`, gated
// to director + finance only.
func registerFinance(api, sensitiveAPI huma.API, svc *financeusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-finance",
		Method:      http.MethodGet,
		Path:        "/students/{id}/finance",
		Summary:     "A student's invoices, payments and balance",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentFinanceInput) (*studentFinanceOutput, error) {
		p, sid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		now := time.Now()
		invoices, err := svc.ListInvoices(ctx, p.OrgID, sid)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		payments, err := svc.ListPayments(ctx, p.OrgID, sid)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		bal, err := svc.StudentBalance(ctx, p.OrgID, sid)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &studentFinanceOutput{}
		out.Body.Balance = bal
		out.Body.Invoices = make([]invoiceResponse, 0, len(invoices))
		for _, iv := range invoices {
			out.Body.Invoices = append(out.Body.Invoices, toInvoiceResponse(iv, now))
		}
		out.Body.Payments = make([]paymentResponse, 0, len(payments))
		for _, pay := range payments {
			out.Body.Payments = append(out.Body.Payments, toPaymentResponse(pay))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "invoice-create",
		Method:        http.MethodPost,
		Path:          "/students/{id}/invoices",
		Summary:       "Create an invoice for a student",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createInvoiceInput) (*invoiceOutput, error) {
		p, sid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		due, derr := parseOptDate(in.Body.DueDate)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("dueDate must be YYYY-MM-DD")
		}
		enr, derr := parseOptUUID(in.Body.Enrollment)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid enrollmentId")
		}
		gidInv, gierr := parseOptUUID(in.Body.GroupID)
		if gierr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid groupId")
		}
		iv, err := svc.CreateInvoice(ctx, p.OrgID, sid, financeusecase.InvoiceInput{
			GroupID:      gidInv,
			EnrollmentID: enr,
			Amount:       in.Body.Amount,
			DueDate:      due,
			Period:       in.Body.Period,
			Note:         in.Body.Note,
		})
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &invoiceOutput{Body: toInvoiceResponse(iv, time.Now())}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "invoice-delete",
		Method:        http.MethodDelete,
		Path:          "/invoices/{id}",
		Summary:       "Delete an invoice (cascades its payments)",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *financeIDInput) (*deleteFinanceOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteInvoice(ctx, p.OrgID, id); err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &deleteFinanceOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "invoices-generate-monthly",
		Method:      http.MethodPost,
		Path:        "/finance/invoices/generate",
		Summary:     "Bill every active group member for a month at their course price (idempotent)",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *generateInvoicesInput) (*generateInvoicesOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		created, err := svc.GenerateMonthlyInvoices(ctx, p.OrgID, in.Body.Period)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &generateInvoicesOutput{Body: struct {
			Created int64 `json:"created"`
		}{Created: created}}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "payment-record",
		Method:        http.MethodPost,
		Path:          "/invoices/{id}/payments",
		Summary:       "Record a payment against an invoice",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordPaymentInput) (*paymentOutput, error) {
		p, iid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		paidAt, derr := parseOptDate(in.Body.PaidAt)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("paidAt must be YYYY-MM-DD")
		}
		pay, err := svc.RecordPayment(ctx, p.OrgID, iid, financeusecase.PaymentInput{
			Amount: in.Body.Amount,
			Method: in.Body.Method,
			PaidAt: paidAt,
			Note:   in.Body.Note,
		})
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &paymentOutput{Body: toPaymentResponse(pay)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "payment-refund",
		Method:        http.MethodPost,
		Path:          "/invoices/{id}/refund",
		Summary:       "Refund money against an invoice (recorded as a negative payment)",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordPaymentInput) (*paymentOutput, error) {
		p, iid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		paidAt, derr := parseOptDate(in.Body.PaidAt)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("paidAt must be YYYY-MM-DD")
		}
		pay, err := svc.RefundPayment(ctx, p.OrgID, iid, financeusecase.PaymentInput{
			Amount: in.Body.Amount,
			Method: in.Body.Method,
			PaidAt: paidAt,
			Note:   in.Body.Note,
		})
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &paymentOutput{Body: toPaymentResponse(pay)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "payment-delete",
		Method:        http.MethodDelete,
		Path:          "/payments/{id}",
		Summary:       "Delete a payment (reverses it from the invoice)",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *financeIDInput) (*deleteFinanceOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeletePayment(ctx, p.OrgID, id); err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &deleteFinanceOutput{}, nil
	})

	Register(sensitiveAPI, BearerOperation(huma.Operation{
		OperationID: "finance-summary",
		Method:      http.MethodGet,
		Path:        "/finance/summary",
		Summary:     "Center money snapshot: today/month income and total debt",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *financeSummaryInput) (*financeSummaryOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		sum, err := svc.Summary(ctx, p.OrgID)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &financeSummaryOutput{}
		out.Body.TodayIncome = sum.TodayIncome
		out.Body.MonthIncome = sum.MonthIncome
		out.Body.TotalDebt = sum.TotalDebt
		out.Body.TodayExpense = sum.TodayExpense
		out.Body.MonthExpense = sum.MonthExpense
		out.Body.MonthProfit = sum.MonthProfit()
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "finance-debtors",
		Method:      http.MethodGet,
		Path:        "/finance/debtors",
		Summary:     "Students with an outstanding balance (highest first)",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *financeSummaryInput) (*debtorsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		ds, err := svc.Debtors(ctx, p.OrgID)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &debtorsOutput{}
		for _, d := range ds {
			out.Body = append(out.Body, struct {
				StudentID string `json:"studentId"`
				Name      string `json:"name"`
				Balance   int64  `json:"balance"`
			}{StudentID: d.StudentID.String(), Name: d.Name, Balance: d.Balance})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "group-finance",
		Method:      http.MethodGet,
		Path:        "/groups/{id}/finance",
		Summary:     "Group roster billing state for a month (invoiced/paid per student)",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *groupFinanceInput) (*groupFinanceOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		gid, gerr := uuid.Parse(in.ID)
		if gerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid group id")
		}
		period := strings.TrimSpace(in.Month)
		if period == "" {
			period = time.Now().Format("2006-01")
		}
		rows, err := svc.GroupFinance(ctx, p.OrgID, gid, period)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &groupFinanceOutput{}
		out.Body.Period = period
		out.Body.Students = make([]groupFinanceRow, 0, len(rows))
		for _, r := range rows {
			out.Body.Students = append(out.Body.Students, groupFinanceRow{
				StudentID: r.StudentID.String(),
				Name:      r.Name,
				Invoiced:  r.Invoiced,
				Paid:      r.Paid,
				Attended:  r.Attended,
			})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "finance-alerts",
		Method:      http.MethodGet,
		Path:        "/finance/alerts",
		Summary:     "Billing alerts: overdue invoices + attending-without-invoice (grace passed)",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *financeSummaryInput) (*paymentAlertsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		overdue, grace, err := svc.PaymentAlerts(ctx, p.OrgID, time.Now().Format("2006-01"))
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &paymentAlertsOutput{}
		out.Body.OverdueInvoices = make([]overdueInvoiceRow, 0, len(overdue))
		for _, o := range overdue {
			row := overdueInvoiceRow{
				StudentID:   o.StudentID.String(),
				StudentName: o.StudentName,
				Balance:     o.Balance(),
				Period:      o.Period,
			}
			if o.GroupID != nil {
				row.GroupID = o.GroupID.String()
			}
			if o.DueDate != nil {
				d := o.DueDate.Format("2006-01-02")
				row.DueDate = &d
			}
			out.Body.OverdueInvoices = append(out.Body.OverdueInvoices, row)
		}
		out.Body.GraceOverdue = make([]graceOverdueRow, 0, len(grace))
		for _, g := range grace {
			out.Body.GraceOverdue = append(out.Body.GraceOverdue, graceOverdueRow{
				StudentID:   g.StudentID.String(),
				StudentName: g.StudentName,
				GroupID:     g.GroupID.String(),
				GroupName:   g.GroupName,
				Attended:    g.Attended,
			})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "billing-settings-get",
		Method:      http.MethodGet,
		Path:        "/finance/settings",
		Summary:     "Center billing rules (grace lessons)",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *financeSummaryInput) (*billingSettingsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		n, err := svc.GraceLessons(ctx, p.OrgID)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &billingSettingsOutput{Body: billingSettingsBody{GraceLessons: n}}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "billing-settings-update",
		Method:      http.MethodPut,
		Path:        "/finance/settings",
		Summary:     "Update center billing rules",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *billingSettingsInput) (*billingSettingsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		if err := svc.SetGraceLessons(ctx, p.OrgID, in.Body.GraceLessons); err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &billingSettingsOutput{Body: in.Body}, nil
	})

	Register(sensitiveAPI, BearerOperation(huma.Operation{
		OperationID: "expenses-list",
		Method:      http.MethodGet,
		Path:        "/finance/expenses",
		Summary:     "Operating expenses in a date range (default: this month) + category breakdown",
		Tags:        []string{"finance"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *listExpensesInput) (*expensesListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		from, to, derr := parseFinanceRange(in.From, in.To)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("from/to must be YYYY-MM-DD")
		}
		list, err := svc.ListExpenses(ctx, p.OrgID, from, to)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		cats, err := svc.ExpensesByCategory(ctx, p.OrgID, from, to)
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		out := &expensesListOutput{}
		out.Body.Expenses = make([]expenseResponse, 0, len(list))
		out.Body.ByCategory = make([]expenseCategoryTotal, 0, len(cats))
		for _, e := range list {
			out.Body.Expenses = append(out.Body.Expenses, toExpenseResponse(e))
			out.Body.Total += e.Amount
		}
		for _, c := range cats {
			out.Body.ByCategory = append(out.Body.ByCategory, expenseCategoryTotal{Category: c.Category, Total: c.Total})
		}
		return out, nil
	})

	Register(sensitiveAPI, BearerOperation(huma.Operation{
		OperationID:   "expense-create",
		Method:        http.MethodPost,
		Path:          "/finance/expenses",
		Summary:       "Record an operating expense",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createExpenseInput) (*expenseOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		spentAt, derr := parseOptDate(in.Body.SpentAt)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("spentAt must be YYYY-MM-DD")
		}
		e, err := svc.CreateExpense(ctx, p.OrgID, financeusecase.ExpenseInput{
			Category: in.Body.Category,
			Amount:   in.Body.Amount,
			SpentAt:  spentAt,
			Note:     in.Body.Note,
		})
		if err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &expenseOutput{Body: toExpenseResponse(e)}, nil
	})

	Register(sensitiveAPI, BearerOperation(huma.Operation{
		OperationID:   "expense-delete",
		Method:        http.MethodDelete,
		Path:          "/finance/expenses/{id}",
		Summary:       "Delete an expense",
		Tags:          []string{"finance"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *financeIDInput) (*deleteFinanceOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteExpense(ctx, p.OrgID, id); err != nil {
			return nil, mapFinanceError(LangFromContext(ctx), err, log)
		}
		return &deleteFinanceOutput{}, nil
	})
}

func toExpenseResponse(e entity.Expense) expenseResponse {
	return expenseResponse{
		ID:       e.ID.String(),
		Category: e.Category,
		Amount:   e.Amount,
		SpentAt:  e.SpentAt.Format("2006-01-02"),
		Note:     e.Note,
	}
}

// parseFinanceRange parses optional from/to YYYY-MM-DD; empty → zero time (service defaults to month).
func parseFinanceRange(from, to string) (time.Time, time.Time, error) {
	var f, t time.Time
	if from != "" {
		p, err := time.Parse("2006-01-02", from)
		if err != nil {
			return f, t, err
		}
		f = p
	}
	if to != "" {
		p, err := time.Parse("2006-01-02", to)
		if err != nil {
			return f, t, err
		}
		t = p
	}
	return f, t, nil
}

func toInvoiceResponse(i entity.Invoice, now time.Time) invoiceResponse {
	r := invoiceResponse{
		ID:         i.ID.String(),
		StudentID:  i.StudentID.String(),
		Amount:     i.Amount,
		PaidAmount: i.PaidAmount,
		Balance:    i.Balance(),
		Status:     i.Status(now),
		Period:     i.Period,
		Note:       i.Note,
		CreatedAt:  i.CreatedAt.UTC().Format(time.RFC3339),
	}
	if i.GroupID != nil {
		r.GroupID = i.GroupID.String()
	}
	if i.DueDate != nil {
		s := i.DueDate.Format("2006-01-02")
		r.DueDate = &s
	}
	return r
}

func toPaymentResponse(p entity.Payment) paymentResponse {
	return paymentResponse{
		ID:        p.ID.String(),
		InvoiceID: p.InvoiceID.String(),
		StudentID: p.StudentID.String(),
		Amount:    p.Amount,
		Method:    p.Method,
		Note:      p.Note,
		PaidAt:    p.PaidAt.UTC().Format(time.RFC3339),
	}
}

var msgInvoiceNotFound = i18n.Message{UZ: "Hisob topilmadi.", RU: "Счёт не найден.", EN: "Invoice not found."}

func mapFinanceError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, repo.ErrOverpay):
		return huma.Error422UnprocessableEntity("To'lov qoldiqdan oshib ketdi — avval hisob summasini tekshiring")
	case errors.Is(err, financeusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgInvoiceNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("finance: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}
