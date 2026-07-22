// Package points is the gamification engine: it awards XP + coins for real student actions
// (attendance, homework, check-ins) idempotently, and keeps the student's cached totals in sync.
// Other usecases call it as a side-effect after their action succeeds; awards never block the action.
package points

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

type Service struct {
	repo repo.PointsRepository
}

func NewService(r repo.PointsRepository) *Service { return &Service{repo: r} }

// AwardAttendance grants attendance XP/coins once per (group, date).
func (s *Service) AwardAttendance(ctx context.Context, orgID, studentID, groupID uuid.UUID, date time.Time) error {
	ref := groupID.String() + ":" + date.Format("2006-01-02")
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointAttendance, entity.XPAttendance, entity.CoinsAttendance, ref)
	return err
}

// AwardHomeworkSubmit grants submit XP/coins once per assignment (on-time earns more).
func (s *Service) AwardHomeworkSubmit(ctx context.Context, orgID, studentID, assignmentID uuid.UUID, onTime bool) error {
	xp, coins := entity.XPHomeworkLate, 0
	if onTime {
		xp, coins = entity.XPHomeworkOnTime, entity.CoinsHomeworkOnTime
	}
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointHomeworkSubmit, xp, coins, assignmentID.String())
	return err
}

// AwardHomeworkAccept grants the graded score as XP (plus a small coin bonus) once per submission.
func (s *Service) AwardHomeworkAccept(ctx context.Context, orgID, studentID, submissionID uuid.UUID, score int) error {
	if score < 0 {
		score = 0
	}
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointHomeworkAccept, score, entity.CoinsHomeworkAccept, submissionID.String())
	return err
}

// AwardCheckin grants check-in XP/coins once per week.
func (s *Service) AwardCheckin(ctx context.Context, orgID, studentID uuid.UUID, week int) error {
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointCheckin, entity.XPCheckin, entity.CoinsCheckin, strconv.Itoa(week))
	return err
}
