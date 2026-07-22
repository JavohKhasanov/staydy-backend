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

// Leaderboard returns the top students by XP, center-wide or within a group, with the caller's own
// row flagged (meID may be uuid.Nil for a back-office viewer).
func (s *Service) Leaderboard(ctx context.Context, orgID uuid.UUID, groupID *uuid.UUID, meID uuid.UUID) ([]entity.LeaderRow, error) {
	const limit = 100
	var rows []entity.LeaderRow
	var err error
	if groupID != nil {
		rows, err = s.repo.LeaderboardByGroup(ctx, orgID, *groupID, limit)
	} else {
		rows, err = s.repo.Leaderboard(ctx, orgID, limit)
	}
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].StudentID == meID {
			rows[i].IsMe = true
		}
	}
	return rows, nil
}
