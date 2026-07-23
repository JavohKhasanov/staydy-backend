package http

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rs/zerolog"

	financeusecase "github.com/student-success/backend/internal/usecase/finance"
	groupusecase "github.com/student-success/backend/internal/usecase/group"
)

type studentGroupResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Direction    string `json:"direction,omitempty"`
	ScheduleDays string `json:"scheduleDays,omitempty"`
	StartTime    string `json:"startTime,omitempty"`
	EndTime      string `json:"endTime,omitempty"`
}
type studentGroupsOutput struct{ Body []studentGroupResponse }

type studentSelfInvoiceResponse struct {
	ID         string  `json:"id"`
	Period     string  `json:"period,omitempty"`
	Amount     int64   `json:"amount"`
	PaidAmount int64   `json:"paidAmount"`
	DueDate    *string `json:"dueDate,omitempty"`
	Status     string  `json:"status"`
	Note       string  `json:"note,omitempty"`
}
type studentSelfPaymentResponse struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
	Method string `json:"method"`
	PaidAt string `json:"paidAt"`
}
type studentSelfFinanceOutput struct {
	Body struct {
		Balance  int64                    `json:"balance"`
		Invoices []studentSelfInvoiceResponse `json:"invoices"`
		Payments []studentSelfPaymentResponse `json:"payments"`
	}
}

// registerStudentData mounts the student's own groups + finance. Mount on the student group.
func registerStudentData(api huma.API, groups *groupusecase.Service, finance *financeusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-my-groups",
		Method:      http.MethodGet,
		Path:        "/student/groups",
		Summary:     "The student's own groups",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentGroupsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		gs, err := groups.StudentGroups(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student groups failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentGroupsOutput{Body: make([]studentGroupResponse, 0, len(gs))}
		for _, g := range gs {
			out.Body = append(out.Body, studentGroupResponse{
				ID: g.ID.String(), Name: g.Name, Direction: g.Direction,
				ScheduleDays: g.ScheduleDays, StartTime: g.StartTime, EndTime: g.EndTime,
			})
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "student-my-finance",
		Method:      http.MethodGet,
		Path:        "/student/finance",
		Summary:     "The student's own invoices, payments and balance",
		Tags:        []string{"student"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *struct{}) (*studentSelfFinanceOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		invoices, err := finance.ListInvoices(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student invoices failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		payments, err := finance.ListPayments(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student payments failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		balance, err := finance.StudentBalance(ctx, p.OrgID, p.UserID)
		if err != nil {
			log.Error().Err(err).Msg("student balance failed")
			return nil, huma.Error500InternalServerError("internal error")
		}
		out := &studentSelfFinanceOutput{}
		out.Body.Balance = balance
		out.Body.Invoices = make([]studentSelfInvoiceResponse, 0, len(invoices))
		now := time.Now()
		for _, iv := range invoices {
			r := studentSelfInvoiceResponse{
				ID: iv.ID.String(), Period: iv.Period, Amount: iv.Amount, PaidAmount: iv.PaidAmount,
				Note: iv.Note, Status: invoiceStatus(iv.Amount, iv.PaidAmount, iv.DueDate, now),
			}
			if iv.DueDate != nil {
				v := iv.DueDate.Format("2006-01-02")
				r.DueDate = &v
			}
			out.Body.Invoices = append(out.Body.Invoices, r)
		}
		out.Body.Payments = make([]studentSelfPaymentResponse, 0, len(payments))
		for _, pay := range payments {
			out.Body.Payments = append(out.Body.Payments, studentSelfPaymentResponse{
				ID: pay.ID.String(), Amount: pay.Amount, Method: pay.Method,
				PaidAt: pay.PaidAt.Format("2006-01-02"),
			})
		}
		return out, nil
	})
}

func invoiceStatus(amount, paid int64, due *time.Time, now time.Time) string {
	switch {
	case paid >= amount:
		return "paid"
	case paid > 0:
		return "partial"
	case due != nil && due.Before(now):
		return "overdue"
	default:
		return "unpaid"
	}
}
