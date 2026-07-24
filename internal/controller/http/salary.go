package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
	salaryusecase "github.com/student-success/backend/internal/usecase/salary"
)

type salaryRuleResponse struct {
	TeacherID string `json:"teacherId"`
	Kind      string `json:"kind"`
	Rate      int64  `json:"rate"`
	Base      int64  `json:"base"`
}
type salaryGroupBasisResponse struct {
	GroupID   string `json:"groupId"`
	GroupName string `json:"groupName"`
	Kind      string `json:"kind"`
	Rate      int64  `json:"rate"`
	Override  bool   `json:"override"`
	Students  int64  `json:"students"`
	Lessons   int64  `json:"lessons"`
	Revenue   int64  `json:"revenue"`
	Amount    int64  `json:"amount"`
}
type salaryBasisResponse struct {
	Kind     string                     `json:"kind"`
	Rate     int64                      `json:"rate"`
	Base     int64                      `json:"base"`
	Lessons  int64                      `json:"lessons"`
	Students int64                      `json:"students"`
	Revenue  int64                      `json:"revenue"`
	Groups   []salaryGroupBasisResponse `json:"groups"`
	Gross    int64                      `json:"gross"`
}
type salarySlipResponse struct {
	ID          string  `json:"id"`
	TeacherID   string  `json:"teacherId"`
	TeacherName string  `json:"teacherName,omitempty"`
	PeriodStart string  `json:"periodStart"`
	PeriodEnd   string  `json:"periodEnd"`
	Gross       int64   `json:"gross"`
	Bonus       int64   `json:"bonus"`
	Deduction   int64   `json:"deduction"`
	Net         int64   `json:"net"`
	Status      string  `json:"status"`
	Note        string  `json:"note,omitempty"`
	PaidAt      *string `json:"paidAt,omitempty"`
}

type teacherIDPathInput struct {
	ID string `path:"id" format:"uuid"`
}
type salaryGroupRuleInput struct {
	GroupID string `json:"groupId" format:"uuid"`
	Kind    string `json:"kind" enum:"fixed,per_lesson,per_student,percent_revenue"`
	Rate    int64  `json:"rate" minimum:"0"`
}
type setSalaryRuleInput struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Kind   string                 `json:"kind" enum:"fixed,per_lesson,per_student,percent_revenue"`
		Rate   int64                  `json:"rate" minimum:"0" doc:"so'm for fixed/per_lesson/per_student; percent(0-100) for percent_revenue"`
		Base   int64                  `json:"base,omitempty" minimum:"0" doc:"fixed monthly base added on top of the variable part (hybrid)"`
		Groups []salaryGroupRuleInput `json:"groups,omitempty" doc:"per-group overrides; replaces all existing overrides when present"`
	}
}
type salaryPreviewInput struct {
	ID   string `path:"id" format:"uuid"`
	From string `query:"from" doc:"YYYY-MM-DD (default: month start)"`
	To   string `query:"to" doc:"YYYY-MM-DD (default: month end)"`
}
type listSlipsInput struct {
	From string `query:"from"`
	To   string `query:"to"`
}
type createSlipInput struct {
	Body struct {
		TeacherID   string `json:"teacherId" format:"uuid"`
		PeriodStart string `json:"periodStart" doc:"YYYY-MM-DD"`
		PeriodEnd   string `json:"periodEnd" doc:"YYYY-MM-DD"`
		Gross       int64  `json:"gross" minimum:"0"`
		Bonus       int64  `json:"bonus,omitempty" minimum:"0"`
		Deduction   int64  `json:"deduction,omitempty" minimum:"0"`
		Note        string `json:"note,omitempty" maxLength:"500"`
	}
}

type salaryRuleOutput struct{ Body salaryRuleResponse }
type salaryBasisOutput struct{ Body salaryBasisResponse }
type salarySlipOutput struct{ Body salarySlipResponse }
type salarySlipsListOutput struct{ Body []salarySlipResponse }

func registerSalary(api huma.API, svc *salaryusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "salary-rule-get",
		Method:      http.MethodGet,
		Path:        "/teachers/{id}/salary-rule",
		Summary:     "Get a teacher's salary rule",
		Tags:        []string{"salary"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*salaryRuleOutput, error) {
		p, tid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		rule, err := svc.GetRule(ctx, p.OrgID, tid)
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		return &salaryRuleOutput{Body: toSalaryRuleResponse(rule, tid)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "salary-rule-set",
		Method:      http.MethodPut,
		Path:        "/teachers/{id}/salary-rule",
		Summary:     "Set a teacher's salary rule",
		Tags:        []string{"salary"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *setSalaryRuleInput) (*salaryRuleOutput, error) {
		p, tid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		var groups []salaryusecase.GroupRuleInput
		if in.Body.Groups != nil {
			groups = make([]salaryusecase.GroupRuleInput, 0, len(in.Body.Groups))
			for _, g := range in.Body.Groups {
				gid, perr := uuid.Parse(g.GroupID)
				if perr != nil {
					return nil, huma.Error422UnprocessableEntity("invalid groupId")
				}
				groups = append(groups, salaryusecase.GroupRuleInput{GroupID: gid, Kind: g.Kind, Rate: g.Rate})
			}
		}
		rule, err := svc.SetRule(ctx, p.OrgID, tid, in.Body.Kind, in.Body.Rate, in.Body.Base, groups)
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		return &salaryRuleOutput{Body: toSalaryRuleResponse(rule, tid)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "salary-preview",
		Method:      http.MethodGet,
		Path:        "/teachers/{id}/salary/preview",
		Summary:     "Compute a teacher's gross pay for a period from real data",
		Tags:        []string{"salary"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *salaryPreviewInput) (*salaryBasisOutput, error) {
		p, tid, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		from, to, derr := monthRange(in.From, in.To)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("from/to must be YYYY-MM-DD")
		}
		b, err := svc.Preview(ctx, p.OrgID, tid, from, to)
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		groups := make([]salaryGroupBasisResponse, 0, len(b.Groups))
		for _, g := range b.Groups {
			groups = append(groups, salaryGroupBasisResponse{
				GroupID:   g.GroupID.String(),
				GroupName: g.GroupName,
				Kind:      g.Kind,
				Rate:      g.Rate,
				Override:  g.Override,
				Students:  g.Students,
				Lessons:   g.Lessons,
				Revenue:   g.Revenue,
				Amount:    g.Amount,
			})
		}
		return &salaryBasisOutput{Body: salaryBasisResponse{
			Kind: b.Kind, Rate: b.Rate, Base: b.Base, Lessons: b.Lessons, Students: b.Students, Revenue: b.Revenue, Groups: groups, Gross: b.Gross,
		}}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "salary-slips-list",
		Method:      http.MethodGet,
		Path:        "/salary-slips",
		Summary:     "List salary slips in a period (default: this month)",
		Tags:        []string{"salary"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *listSlipsInput) (*salarySlipsListOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		from, to, derr := parseFinanceRange(in.From, in.To)
		if derr != nil {
			return nil, huma.Error422UnprocessableEntity("from/to must be YYYY-MM-DD")
		}
		slips, err := svc.ListSlips(ctx, p.OrgID, from, to)
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		out := &salarySlipsListOutput{}
		out.Body = make([]salarySlipResponse, 0, len(slips))
		for _, s := range slips {
			out.Body = append(out.Body, toSalarySlipResponse(s))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "salary-slip-create",
		Method:        http.MethodPost,
		Path:          "/salary-slips",
		Summary:       "Create a salary slip (net = gross + bonus − deduction)",
		Tags:          []string{"salary"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createSlipInput) (*salarySlipOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		tid, perr := uuid.Parse(in.Body.TeacherID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid teacherId")
		}
		from, ferr := time.Parse("2006-01-02", in.Body.PeriodStart)
		to, terr := time.Parse("2006-01-02", in.Body.PeriodEnd)
		if ferr != nil || terr != nil {
			return nil, huma.Error422UnprocessableEntity("periodStart/periodEnd must be YYYY-MM-DD")
		}
		slip, err := svc.CreateSlip(ctx, p.OrgID, salaryusecase.CreateSlipInput{
			TeacherID:   tid,
			PeriodStart: from,
			PeriodEnd:   to,
			Gross:       in.Body.Gross,
			Bonus:       in.Body.Bonus,
			Deduction:   in.Body.Deduction,
			Note:        in.Body.Note,
		})
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		return &salarySlipOutput{Body: toSalarySlipResponse(slip)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "salary-slip-pay",
		Method:      http.MethodPatch,
		Path:        "/salary-slips/{id}/pay",
		Summary:     "Mark a salary slip paid",
		Tags:        []string{"salary"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*salarySlipOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		slip, err := svc.MarkPaid(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapSalaryError(err, log)
		}
		return &salarySlipOutput{Body: toSalarySlipResponse(slip)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "salary-slip-delete",
		Method:        http.MethodDelete,
		Path:          "/salary-slips/{id}",
		Summary:       "Delete a salary slip",
		Tags:          []string{"salary"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *teacherIDPathInput) (*noContentOutput, error) {
		p, id, err := orgAndID(ctx, in.ID)
		if err != nil {
			return nil, err
		}
		if err := svc.DeleteSlip(ctx, p.OrgID, id); err != nil {
			return nil, mapSalaryError(err, log)
		}
		return &noContentOutput{}, nil
	})
}

func toSalaryRuleResponse(r entity.SalaryRule, teacherID uuid.UUID) salaryRuleResponse {
	return salaryRuleResponse{TeacherID: teacherID.String(), Kind: r.Kind, Rate: r.Rate, Base: r.Base}
}

func toSalarySlipResponse(s entity.SalarySlip) salarySlipResponse {
	r := salarySlipResponse{
		ID:          s.ID.String(),
		TeacherID:   s.TeacherID.String(),
		TeacherName: s.TeacherName,
		PeriodStart: s.PeriodStart.Format("2006-01-02"),
		PeriodEnd:   s.PeriodEnd.Format("2006-01-02"),
		Gross:       s.Gross,
		Bonus:       s.Bonus,
		Deduction:   s.Deduction,
		Net:         s.Net,
		Status:      s.Status,
		Note:        s.Note,
	}
	if s.PaidAt != nil {
		v := s.PaidAt.UTC().Format(time.RFC3339)
		r.PaidAt = &v
	}
	return r
}

// monthRange parses optional from/to YYYY-MM-DD, defaulting to the current calendar month.
func monthRange(from, to string) (time.Time, time.Time, error) {
	f, t, err := parseFinanceRange(from, to)
	if err != nil {
		return f, t, err
	}
	if f.IsZero() || t.IsZero() {
		now := time.Now()
		f = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		t = f.AddDate(0, 1, -1)
	}
	return f, t, nil
}

func mapSalaryError(err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, salaryusecase.ErrValidation):
		return huma.Error422UnprocessableEntity("ma'lumotlar noto'g'ri")
	case errors.Is(err, salaryusecase.ErrNotFound):
		return huma.Error404NotFound("topilmadi")
	case errors.Is(err, repo.ErrConflict):
		return huma.Error422UnprocessableEntity("to'langan maosh slipini o'chirib bo'lmaydi")
	default:
		log.Error().Err(err).Msg("salary op failed")
		return huma.Error500InternalServerError("internal error")
	}
}
