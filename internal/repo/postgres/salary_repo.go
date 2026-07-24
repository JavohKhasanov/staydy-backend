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

// SalaryRepository persists teacher payroll (rules + slips) and computes a period's basis (done
// lessons / active students / revenue) from the tenant's real data. All RLS-scoped.
type SalaryRepository struct {
	db *postgres.DB
}

func NewSalaryRepository(db *postgres.DB) *SalaryRepository { return &SalaryRepository{db: db} }

func (r *SalaryRepository) GetRule(ctx context.Context, orgID, teacherID uuid.UUID) (entity.SalaryRule, error) {
	var rule entity.SalaryRule
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).GetSalaryRule(ctx, teacherID)
		if e != nil {
			return e
		}
		rule = mapSalaryRule(row)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.SalaryRule{}, repo.ErrNotFound
	}
	return rule, err
}

func (r *SalaryRepository) SetRule(ctx context.Context, orgID, teacherID uuid.UUID, kind string, rate, base int64) (entity.SalaryRule, error) {
	var rule entity.SalaryRule
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).UpsertSalaryRule(ctx, sqlc.UpsertSalaryRuleParams{
			OrgID:      orgID,
			TeacherID:  teacherID,
			Kind:       kind,
			Rate:       rate,
			BaseAmount: base,
		})
		if e != nil {
			return e
		}
		rule = mapSalaryRule(row)
		return nil
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.SalaryRule{}, repo.ErrNotFound
		}
		return entity.SalaryRule{}, err
	}
	return rule, nil
}

// ListGroupRules returns a teacher's per-group salary overrides.
func (r *SalaryRepository) ListGroupRules(ctx context.Context, orgID, teacherID uuid.UUID) ([]entity.SalaryGroupRule, error) {
	var out []entity.SalaryGroupRule
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListSalaryGroupRules(ctx, teacherID)
		if e != nil {
			return e
		}
		out = make([]entity.SalaryGroupRule, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.SalaryGroupRule{
				TeacherID: row.TeacherID, GroupID: row.GroupID, Kind: row.Kind, Rate: row.Rate,
			})
		}
		return nil
	})
	return out, err
}

// ReplaceGroupRules atomically sets a teacher's per-group overrides to exactly rules (deleting any
// not present). An empty slice clears all overrides.
func (r *SalaryRepository) ReplaceGroupRules(ctx context.Context, orgID, teacherID uuid.UUID, rules []repo.GroupRuleParams) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		if e := q.DeleteTeacherGroupRules(ctx, teacherID); e != nil {
			return e
		}
		for _, gr := range rules {
			if _, e := q.InsertSalaryGroupRule(ctx, sqlc.InsertSalaryGroupRuleParams{
				OrgID:     orgID,
				TeacherID: teacherID,
				GroupID:   gr.GroupID,
				Kind:      gr.Kind,
				Rate:      gr.Rate,
			}); e != nil {
				return e
			}
		}
		return nil
	})
}

// GroupBasis returns the per-group counts (active members, done lessons, revenue) for a teacher's
// groups in the period.
func (r *SalaryRepository) GroupBasis(ctx context.Context, orgID, teacherID uuid.UUID, from, to time.Time) ([]entity.GroupBasis, error) {
	var out []entity.GroupBasis
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).TeacherGroupBasis(ctx, sqlc.TeacherGroupBasisParams{
			Column1: teacherID, Column2: dateVal(&from), Column3: dateVal(&to),
		})
		if e != nil {
			return e
		}
		out = make([]entity.GroupBasis, 0, len(rows))
		for _, row := range rows {
			out = append(out, entity.GroupBasis{
				GroupID:   row.GroupID,
				GroupName: row.GroupName,
				Students:  row.Students,
				Lessons:   row.Lessons,
				Revenue:   row.Revenue,
			})
		}
		return nil
	})
	return out, err
}

// Basis computes the period's done-lesson count, active-student count, and revenue for a teacher.
func (r *SalaryRepository) Basis(ctx context.Context, orgID, teacherID uuid.UUID, from, to time.Time) (lessons, students, revenue int64, err error) {
	err = r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var e error
		lessons, e = q.CountTeacherDoneLessons(ctx, sqlc.CountTeacherDoneLessonsParams{
			Column1: teacherID, Column2: dateVal(&from), Column3: dateVal(&to),
		})
		if e != nil {
			return e
		}
		students, e = q.CountTeacherActiveStudents(ctx, teacherID)
		if e != nil {
			return e
		}
		revenue, e = q.TeacherRevenue(ctx, sqlc.TeacherRevenueParams{
			Column1: teacherID, Column2: dateVal(&from), Column3: dateVal(&to),
		})
		return e
	})
	return lessons, students, revenue, err
}

func (r *SalaryRepository) CreateSlip(ctx context.Context, orgID uuid.UUID, p repo.SalarySlipParams) (entity.SalarySlip, error) {
	var slip entity.SalarySlip
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		row, e := sqlc.New(tx).CreateSalarySlip(ctx, sqlc.CreateSalarySlipParams{
			OrgID:       orgID,
			TeacherID:   p.TeacherID,
			PeriodStart: dateVal(&p.PeriodStart),
			PeriodEnd:   dateVal(&p.PeriodEnd),
			Gross:       p.Gross,
			Bonus:       p.Bonus,
			Deduction:   p.Deduction,
			Net:         p.Net,
			Note:        p.Note,
		})
		if e != nil {
			return e
		}
		slip = mapSalarySlip(row)
		return nil
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return entity.SalarySlip{}, repo.ErrNotFound
		}
		return entity.SalarySlip{}, err
	}
	return slip, nil
}

func (r *SalaryRepository) ListSlips(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.SalarySlip, error) {
	var out []entity.SalarySlip
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListSalarySlips(ctx, sqlc.ListSalarySlipsParams{
			PeriodStart: dateVal(&from), PeriodEnd: dateVal(&to),
		})
		if e != nil {
			return e
		}
		out = make([]entity.SalarySlip, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapSalarySlipRow(row))
		}
		return nil
	})
	return out, err
}

func (r *SalaryRepository) MarkPaid(ctx context.Context, orgID, id uuid.UUID) (entity.SalarySlip, error) {
	var slip entity.SalarySlip
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		row, e := q.MarkSalarySlipPaid(ctx, id) // no-op (ErrNoRows) if already paid
		if e != nil {
			return e
		}
		slip = mapSalarySlip(row)
		// Paid salary is an operating expense: record it so Moliya's month expense/profit
		// include payroll (category "maosh"; same tx, so a failed pay records nothing).
		teacherName := ""
		if u, uerr := q.GetUserByID(ctx, row.TeacherID); uerr == nil {
			teacherName = u.FullName
		}
		now := time.Now()
		note := "Maosh: " + teacherName + " (" + row.PeriodStart.Time.Format("2006-01") + ")"
		_, e = q.CreateExpense(ctx, sqlc.CreateExpenseParams{
			OrgID:    orgID,
			Category: "maosh",
			Amount:   row.Net,
			SpentAt:  dateVal(&now),
			Note:     note,
		})
		return e
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.SalarySlip{}, repo.ErrNotFound
	}
	return slip, err
}

func (r *SalaryRepository) DeleteSlip(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, err := sqlc.New(tx).DeleteSalarySlip(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			// Missing, or a paid slip the query refuses to delete (its expense would orphan).
			return repo.ErrConflict
		}
		return nil
	})
}
