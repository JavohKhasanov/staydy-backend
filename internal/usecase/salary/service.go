// Package salary is teacher payroll: a per-teacher rule (fixed | per_lesson | per_student |
// percent_revenue) drives a computed gross for a period, which becomes a salary slip (gross ±
// bonus/deduction = net). Gross is computed from the tenant's real data. center_admin concern.
package salary

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	ErrValidation = errors.New("salary: validation failed")
	ErrNotFound   = errors.New("salary: not found")
)

type Service struct {
	repo repo.SalaryRepository
}

func NewService(r repo.SalaryRepository) *Service { return &Service{repo: r} }

// GetRule returns a teacher's salary rule, or a zero-value fixed rule if none is set yet.
func (s *Service) GetRule(ctx context.Context, orgID, teacherID uuid.UUID) (entity.SalaryRule, error) {
	rule, err := s.repo.GetRule(ctx, orgID, teacherID)
	if errors.Is(err, repo.ErrNotFound) {
		return entity.SalaryRule{TeacherID: teacherID, Kind: entity.SalaryFixed, Rate: 0}, nil
	}
	return rule, err
}

// GroupRuleInput is one per-group override supplied when setting a teacher's rule.
type GroupRuleInput struct {
	GroupID uuid.UUID
	Kind    string
	Rate    int64
}

// SetRule sets a teacher's default rule and (when groupRules is non-nil) replaces their per-group
// overrides. Passing a nil groupRules leaves existing overrides untouched; an empty non-nil slice
// clears them.
func (s *Service) SetRule(ctx context.Context, orgID, teacherID uuid.UUID, kind string, rate, base int64, groupRules []GroupRuleInput) (entity.SalaryRule, error) {
	if !entity.ValidSalaryKind(kind) || rate < 0 || base < 0 {
		return entity.SalaryRule{}, ErrValidation
	}
	for _, g := range groupRules {
		if !entity.ValidSalaryKind(g.Kind) || g.Rate < 0 || g.GroupID == uuid.Nil {
			return entity.SalaryRule{}, ErrValidation
		}
	}
	rule, err := s.repo.SetRule(ctx, orgID, teacherID, kind, rate, base)
	if errors.Is(err, repo.ErrNotFound) {
		return entity.SalaryRule{}, ErrNotFound
	}
	if err != nil {
		return entity.SalaryRule{}, err
	}
	if groupRules != nil {
		params := make([]repo.GroupRuleParams, 0, len(groupRules))
		for _, g := range groupRules {
			// Only persist overrides that actually differ; a group matching the default needs no row.
			params = append(params, repo.GroupRuleParams{GroupID: g.GroupID, Kind: g.Kind, Rate: g.Rate})
		}
		if e := s.repo.ReplaceGroupRules(ctx, orgID, teacherID, params); e != nil {
			return entity.SalaryRule{}, e
		}
	}
	return rule, nil
}

// Preview computes a teacher's gross for [from,to]: the fixed base plus each group's variable pay
// (an override, or the teacher default), applied to that group's real counts.
func (s *Service) Preview(ctx context.Context, orgID, teacherID uuid.UUID, from, to time.Time) (entity.SalaryBasis, error) {
	rule, err := s.GetRule(ctx, orgID, teacherID)
	if err != nil {
		return entity.SalaryBasis{}, err
	}
	overrides, err := s.repo.ListGroupRules(ctx, orgID, teacherID)
	if err != nil {
		return entity.SalaryBasis{}, err
	}
	byGroup := make(map[uuid.UUID]entity.SalaryGroupRule, len(overrides))
	for _, o := range overrides {
		byGroup[o.GroupID] = o
	}
	groups, err := s.repo.GroupBasis(ctx, orgID, teacherID, from, to)
	if err != nil {
		return entity.SalaryBasis{}, err
	}

	gross := rule.Base
	var totLessons, totStudents, totRevenue int64
	out := make([]entity.GroupBasis, 0, len(groups))
	for _, g := range groups {
		kind, rate, override := rule.Kind, rule.Rate, false
		if o, ok := byGroup[g.GroupID]; ok {
			kind, rate, override = o.Kind, o.Rate, true
		}
		g.Kind, g.Rate, g.Override = kind, rate, override
		g.Amount = entity.ComputeVariable(kind, rate, g.Lessons, g.Students, g.Revenue)
		gross += g.Amount
		totLessons += g.Lessons
		totStudents += g.Students
		totRevenue += g.Revenue
		out = append(out, g)
	}

	return entity.SalaryBasis{
		Kind:     rule.Kind,
		Rate:     rule.Rate,
		Base:     rule.Base,
		Lessons:  totLessons,
		Students: totStudents,
		Revenue:  totRevenue,
		Groups:   out,
		Gross:    gross,
	}, nil
}

type CreateSlipInput struct {
	TeacherID   uuid.UUID
	PeriodStart time.Time
	PeriodEnd   time.Time
	Gross       int64
	Bonus       int64
	Deduction   int64
	Note        string
}

func (s *Service) CreateSlip(ctx context.Context, orgID uuid.UUID, in CreateSlipInput) (entity.SalarySlip, error) {
	if in.TeacherID == uuid.Nil || in.PeriodStart.IsZero() || in.PeriodEnd.IsZero() || in.Gross < 0 {
		return entity.SalarySlip{}, ErrValidation
	}
	net := in.Gross + in.Bonus - in.Deduction
	if net < 0 {
		net = 0
	}
	slip, err := s.repo.CreateSlip(ctx, orgID, repo.SalarySlipParams{
		TeacherID:   in.TeacherID,
		PeriodStart: in.PeriodStart,
		PeriodEnd:   in.PeriodEnd,
		Gross:       in.Gross,
		Bonus:       in.Bonus,
		Deduction:   in.Deduction,
		Net:         net,
		Note:        in.Note,
	})
	if errors.Is(err, repo.ErrNotFound) {
		return entity.SalarySlip{}, ErrNotFound
	}
	return slip, err
}

func (s *Service) ListSlips(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.SalarySlip, error) {
	if from.IsZero() || to.IsZero() {
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		to = from.AddDate(0, 1, -1)
	}
	return s.repo.ListSlips(ctx, orgID, from, to)
}

func (s *Service) MarkPaid(ctx context.Context, orgID, id uuid.UUID) (entity.SalarySlip, error) {
	slip, err := s.repo.MarkPaid(ctx, orgID, id)
	if errors.Is(err, repo.ErrNotFound) {
		return entity.SalarySlip{}, ErrNotFound
	}
	return slip, err
}

func (s *Service) DeleteSlip(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.DeleteSlip(ctx, orgID, id)
}
