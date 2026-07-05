package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/repo"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// LeadRepository persists the sales funnel. leads is RLS-protected, so every call runs inside the
// tenant scope.
type LeadRepository struct {
	db *postgres.DB
}

func NewLeadRepository(db *postgres.DB) *LeadRepository {
	return &LeadRepository{db: db}
}

func (r *LeadRepository) List(ctx context.Context, orgID uuid.UUID) ([]entity.Lead, error) {
	var out []entity.Lead
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		rows, e := sqlc.New(tx).ListLeads(ctx, orgID)
		if e != nil {
			return e
		}
		out = make([]entity.Lead, 0, len(rows))
		for _, row := range rows {
			out = append(out, mapLead(row))
		}
		return nil
	})
	return out, err
}

func (r *LeadRepository) Get(ctx context.Context, orgID, id uuid.UUID) (entity.Lead, error) {
	var row sqlc.Lead
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).GetLead(ctx, sqlc.GetLeadParams{OrgID: orgID, ID: id})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Lead{}, repo.ErrNotFound
		}
		return entity.Lead{}, err
	}
	return mapLead(row), nil
}

func (r *LeadRepository) Create(ctx context.Context, orgID uuid.UUID, p repo.LeadParams) (entity.Lead, error) {
	var row sqlc.Lead
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).CreateLead(ctx, sqlc.CreateLeadParams{
			OrgID:      orgID,
			Name:       p.Name,
			Phone:      textVal(p.Phone),
			Email:      textVal(p.Email),
			Source:     p.Source,
			Stage:      p.Stage,
			Interest:   p.Interest,
			Note:       p.Note,
			AssignedTo: nullableUUID(p.AssignedTo),
		})
		return e
	})
	if err != nil {
		return entity.Lead{}, err
	}
	return mapLead(row), nil
}

func (r *LeadRepository) Update(ctx context.Context, orgID, id uuid.UUID, p repo.LeadParams) (entity.Lead, error) {
	var row sqlc.Lead
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).UpdateLead(ctx, sqlc.UpdateLeadParams{
			OrgID:      orgID,
			ID:         id,
			Name:       p.Name,
			Phone:      textVal(p.Phone),
			Email:      textVal(p.Email),
			Source:     p.Source,
			Stage:      p.Stage,
			Interest:   p.Interest,
			Note:       p.Note,
			AssignedTo: nullableUUID(p.AssignedTo),
		})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Lead{}, repo.ErrNotFound
		}
		return entity.Lead{}, err
	}
	return mapLead(row), nil
}

func (r *LeadRepository) SetStage(ctx context.Context, orgID, id uuid.UUID, stage string) (entity.Lead, error) {
	var row sqlc.Lead
	err := r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		var e error
		row, e = sqlc.New(tx).SetLeadStage(ctx, sqlc.SetLeadStageParams{OrgID: orgID, ID: id, Stage: stage})
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Lead{}, repo.ErrNotFound
		}
		return entity.Lead{}, err
	}
	return mapLead(row), nil
}

func (r *LeadRepository) MarkConverted(ctx context.Context, orgID, id, studentID uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).MarkLeadConverted(ctx, sqlc.MarkLeadConvertedParams{
			OrgID:     orgID,
			ID:        id,
			StudentID: nullableUUID(&studentID),
		})
	})
}

func (r *LeadRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return r.db.WithTenant(ctx, orgID.String(), func(tx pgx.Tx) error {
		return sqlc.New(tx).DeleteLead(ctx, sqlc.DeleteLeadParams{OrgID: orgID, ID: id})
	})
}

func mapLead(l sqlc.Lead) entity.Lead {
	return entity.Lead{
		ID:         l.ID,
		OrgID:      l.OrgID,
		Name:       l.Name,
		Phone:      textToString(l.Phone),
		Email:      textToString(l.Email),
		Source:     l.Source,
		Stage:      l.Stage,
		Interest:   l.Interest,
		Note:       l.Note,
		AssignedTo: uuidToPtr(l.AssignedTo),
		StudentID:  uuidToPtr(l.StudentID),
		CreatedAt:  l.CreatedAt.Time,
		UpdatedAt:  l.UpdatedAt.Time,
	}
}
