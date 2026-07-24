// Package maintenance orchestrates the scheduled cross-tenant jobs: a daily risk recompute for
// every center (so time-based trigger rules fire without a triggering write) and, on the weekly
// run, pushing the check-in survey to every linked student via the bot.
package maintenance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// slaDays is how long an unresolved at-risk task may sit before it escalates to the director.
const slaDays = 3

// OrgLister lists every center (cross-tenant; NON-RLS).
type OrgLister interface {
	AllOrgIDs(ctx context.Context) ([]uuid.UUID, error)
}

// Recomputer re-runs risk for all students in one org.
type Recomputer interface {
	RecomputeAll(ctx context.Context, orgID uuid.UUID) (int, error)
}

// Broadcaster pushes bot notifications: the weekly survey reminder (cross-tenant) and per-org
// homework deadline reminders.
type Broadcaster interface {
	BroadcastWeeklySurvey(ctx context.Context) (int, error)
	RemindDueHomework(ctx context.Context, orgID uuid.UUID) (int, error)
}

// Escalator raises overdue at-risk tasks to the director as in-app notifications (per org).
type Escalator interface {
	EscalateStaleTasks(ctx context.Context, orgID uuid.UUID, cutoff time.Time) (int, error)
}

type Service struct {
	orgs     OrgLister
	students Recomputer
	bot      Broadcaster
	escalate Escalator
	log      zerolog.Logger
}

func NewService(orgs OrgLister, students Recomputer, bot Broadcaster, escalate Escalator, log zerolog.Logger) *Service {
	return &Service{orgs: orgs, students: students, bot: bot, escalate: escalate, log: log}
}

// EscalateAllTenants raises every center's overdue intervention tasks (older than slaDays) to its
// director. now is passed in so the daily job is deterministic/testable.
func (s *Service) EscalateAllTenants(ctx context.Context, now time.Time) {
	orgIDs, err := s.orgs.AllOrgIDs(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: escalation org list")
		return
	}
	cutoff := now.AddDate(0, 0, -slaDays)
	total := 0
	for _, orgID := range orgIDs {
		n, err := s.escalate.EscalateStaleTasks(ctx, orgID, cutoff)
		if err != nil {
			s.log.Error().Err(err).Str("org", orgID.String()).Msg("scheduler: escalation org")
			continue
		}
		total += n
	}
	if total > 0 {
		s.log.Info().Int("notifications", total).Msg("scheduler: SLA escalations sent")
	}
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

// RemindHomeworkAllTenants pushes homework-deadline reminders across every center. Runs frequently
// (every ~20 min); dedupe lives in the query (only unreminded, in-window assignments).
func (s *Service) RemindHomeworkAllTenants(ctx context.Context) {
	orgIDs, err := s.orgs.AllOrgIDs(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: homework reminder org list")
		return
	}
	total := 0
	for _, orgID := range orgIDs {
		n, err := s.bot.RemindDueHomework(ctx, orgID)
		if err != nil {
			s.log.Error().Err(err).Str("org", orgID.String()).Msg("scheduler: homework reminder org")
			continue
		}
		total += n
	}
	if total > 0 {
		s.log.Info().Int("pushes", total).Msg("scheduler: homework deadline reminders sent")
	}
}

// RunDaily recomputes every tenant (catches time-based trigger rules).
func (s *Service) RunDaily(ctx context.Context) {
	n, err := s.RecomputeAllTenants(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("scheduler: daily recompute")
		return
	}
	s.log.Info().Int("students", n).Msg("scheduler: daily recompute done")
	s.EscalateAllTenants(ctx, time.Now())
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
