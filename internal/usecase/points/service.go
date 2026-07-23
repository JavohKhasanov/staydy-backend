// Package points is the gamification engine: it awards XP for real student actions (attendance,
// homework grade, check-in, exam, manual) idempotently, derives coins from XP at a level-scaled
// rate, and keeps the student's cached totals in sync. Amounts come from the center's config.
package points

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

// ErrValidation signals invalid gamification settings.
var ErrValidation = errors.New("points: validation failed")

type Service struct {
	repo repo.PointsRepository
}

func NewService(r repo.PointsRepository) *Service { return &Service{repo: r} }

// Config returns a center's gamification settings (falling back to defaults on error).
func (s *Service) Config(ctx context.Context, orgID uuid.UUID) entity.GamificationConfig {
	cfg, err := s.repo.GetConfig(ctx, orgID)
	if err != nil {
		return entity.DefaultGamification()
	}
	return cfg
}

func (s *Service) SetConfig(ctx context.Context, orgID uuid.UUID, cfg entity.GamificationConfig) error {
	if cfg.LevelSize < 1 {
		return ErrValidation
	}
	if cfg.XPAttend < 0 || cfg.XPLate < 0 || cfg.XPHomeworkMax < 0 || cfg.XPCheckin < 0 ||
		cfg.CoinBase < 0 || cfg.CoinStep < 0 {
		return ErrValidation
	}
	return s.repo.SetConfig(ctx, orgID, cfg)
}

// AwardAttendance grants XP by attendance status (on-time > late > absent), once per group+date.
func (s *Service) AwardAttendance(ctx context.Context, orgID, studentID, groupID uuid.UUID, date time.Time, status string) error {
	cfg := s.Config(ctx, orgID)
	xp := 0
	switch status {
	case entity.AttendancePresent:
		xp = cfg.XPAttend
	case entity.AttendanceLate, entity.AttendanceExcused:
		xp = cfg.XPLate
	}
	if xp <= 0 {
		return nil
	}
	ref := groupID.String() + ":" + date.Format("2006-01-02")
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointAttendance, xp, ref, cfg)
	return err
}

// AwardHomework grants the teacher's graded XP (clamped to the configured max) once per submission.
func (s *Service) AwardHomework(ctx context.Context, orgID, studentID, submissionID uuid.UUID, xp int) error {
	cfg := s.Config(ctx, orgID)
	if xp < 0 {
		xp = 0
	}
	if xp > cfg.XPHomeworkMax {
		xp = cfg.XPHomeworkMax
	}
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointHomeworkAccept, xp, submissionID.String(), cfg)
	return err
}

// AwardCheckin grants check-in XP once per calendar week.
func (s *Service) AwardCheckin(ctx context.Context, orgID, studentID uuid.UUID, week int) error {
	cfg := s.Config(ctx, orgID)
	_, err := s.repo.Award(ctx, orgID, studentID, entity.PointCheckin, cfg.XPCheckin, strconv.Itoa(week), cfg)
	return err
}

// AwardManual grants a one-off XP amount from a teacher/admin (exam pass, manual bonus). A unique
// ref keeps each award distinct (repeatable, not idempotent).
func (s *Service) AwardManual(ctx context.Context, orgID, studentID uuid.UUID, kind string, xp int, ref string) error {
	if kind != entity.PointExam {
		kind = entity.PointManual
	}
	cfg := s.Config(ctx, orgID)
	_, err := s.repo.Award(ctx, orgID, studentID, kind, xp, ref, cfg)
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
