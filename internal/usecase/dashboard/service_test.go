package dashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

type fakeRepo struct {
	data repo.DashboardData
	err  error
}

func (f *fakeRepo) Load(context.Context, uuid.UUID) (repo.DashboardData, error) {
	return f.data, f.err
}

func TestGet_DerivesKPIs(t *testing.T) {
	svc := NewService(&fakeRepo{data: repo.DashboardData{
		TierCounts: map[string]int{"Green": 5, "Yellow": 3, "Red": 2},
		Groups:     []repo.GroupRisk{{Group: "Frontend", StudentCount: 4, AvgRisk: 60}},
		HighRisk:   []entity.Student{{ID: uuid.New(), Name: "Ali"}},
	}})

	snap, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Total != 10 {
		t.Errorf("Total = %d, want 10", snap.Total)
	}
	if snap.AtRisk != 5 {
		t.Errorf("AtRisk = %d, want 5 (yellow+red)", snap.AtRisk)
	}
	if snap.Green != 5 || snap.Yellow != 3 || snap.Red != 2 {
		t.Errorf("tier counts wrong: g=%d y=%d r=%d", snap.Green, snap.Yellow, snap.Red)
	}
}

func TestGet_EmptyTenant_NonNilSlices(t *testing.T) {
	// A brand-new tenant with no students must still produce a stable, all-zero, non-null
	// JSON shape — the UI binds to arrays and breaks on null.
	svc := NewService(&fakeRepo{data: repo.DashboardData{TierCounts: map[string]int{}}})

	snap, err := svc.Get(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Total != 0 || snap.AtRisk != 0 {
		t.Errorf("empty tenant should be all zero, got total=%d atRisk=%d", snap.Total, snap.AtRisk)
	}
	if snap.Groups == nil || snap.Obstacles == nil || snap.HighRisk == nil || snap.MotivationByWeek == nil {
		t.Error("slices must be non-nil for a stable JSON response")
	}
}

func TestGet_PropagatesRepoError(t *testing.T) {
	wantErr := errors.New("db down")
	svc := NewService(&fakeRepo{err: wantErr})

	if _, err := svc.Get(context.Background(), uuid.New()); !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}
