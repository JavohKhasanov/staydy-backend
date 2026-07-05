// Package maintenance orchestrates the scheduled cross-tenant jobs: a daily risk recompute for
// every center (so time-based trigger rules fire without a triggering write) and, on the weekly
// run, pushing the check-in survey to every linked student via the bot.
package maintenance

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// OrgLister lists every center (cross-tenant; NON-RLS).
type OrgLister interface {
	AllOrgIDs(ctx context.Context) ([]uuid.UUID, error)
}

// Recomputer re-runs risk for all students in one org.
type Recomputer interface {
	RecomputeAll(ctx context.Context, orgID uuid.UUID) (int, error)
}

// Broadcaster pushes the weekly survey to every linked student.
type Broadcaster interface {
	BroadcastWeeklySurvey(ctx context.Context) (int, error)
}

type Service struct {
	orgs     OrgLister
	students Recomputer
	bot      Broadcaster
	log      zerolog.Logger
}

func NewService(orgs OrgLister, students Recomputer, bot Broadcaster, log zerolog.Logger) *Service {
	return &Service{orgs: orgs, students: students, bot: bot, log: log}
}

// RecomputeAllTenants re-runs risk for every student in every center. A failure in one center is
// logged and skipped so the rest still run.
func (s *Service) RecomputeAllTenants(ctx context.Context) (int, error) {
	orgIDs, err := s.orgs.AllOrgIDs(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, orgID := range orgIDs {
		n, err := s.students.RecomputeAll(ctx, orgID)
		if err != nil {
			s.log.Error().Err(err).Str("org", orgID.String()).Msg("scheduler: recompute org")
			continue
		}
		total += n
	}
	return total, nil
}

// RunDaily recomputes every tenant (catches time-based trigger rules).
func (s *Service) RunDaily(ctx context.Context) {
	n, err := s.RecomputeAllTenants(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: daily recompute")
		return
	}
	s.log.Info().Int("students", n).Msg("scheduler: daily recompute done")
}

// RunWeekly recomputes every tenant and then pushes the weekly survey to all linked students.
func (s *Service) RunWeekly(ctx context.Context) {
	s.RunDaily(ctx)
	sent, err := s.bot.BroadcastWeeklySurvey(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: weekly survey broadcast")
		return
	}
	s.log.Info().Int("chats", sent).Msg("scheduler: weekly survey sent")
}
