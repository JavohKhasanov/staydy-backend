// Package lesson is the business logic for scheduled class sessions (the timetable). center_admin
// concern. Additive: attendance is not reworked here.
package lesson

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo"
)

var (
	// ErrValidation signals invalid input (zero date or unknown status).
	ErrValidation = errors.New("lesson: validation failed")
	// ErrRoomBusy means the chosen room is already booked at an overlapping time on that date.
	ErrRoomBusy = errors.New("lesson: room already booked at that time")
)

type Service struct {
	repo repo.LessonRepository
}

func NewService(r repo.LessonRepository) *Service { return &Service{repo: r} }

func (s *Service) List(ctx context.Context, orgID uuid.UUID, from, to time.Time) ([]entity.Lesson, error) {
	return s.repo.List(ctx, orgID, from, to)
}

type Input struct {
	GroupID   *uuid.UUID
	TeacherID *uuid.UUID
	Date      time.Time
	StartTime string
	EndTime   string
	Room      string
	RoomID    *uuid.UUID
	Topic     string
	Status    string
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, in Input) (entity.Lesson, error) {
	p, err := s.normalize(in)
	if err != nil {
		return entity.Lesson{}, err
	}
	if err := s.checkRoom(ctx, orgID, p, uuid.Nil); err != nil {
		return entity.Lesson{}, err
	}
	return s.repo.Create(ctx, orgID, p)
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in Input) (entity.Lesson, error) {
	p, err := s.normalize(in)
	if err != nil {
		return entity.Lesson{}, err
	}
	if err := s.checkRoom(ctx, orgID, p, id); err != nil {
		return entity.Lesson{}, err
	}
	return s.repo.Update(ctx, orgID, id, p)
}

// checkRoom rejects a lesson that double-books a room (only when a room + both times are set).
func (s *Service) checkRoom(ctx context.Context, orgID uuid.UUID, p repo.LessonParams, excludeID uuid.UUID) error {
	if p.RoomID == nil || p.StartTime == "" || p.EndTime == "" {
		return nil
	}
	n, err := s.repo.CountRoomConflicts(ctx, orgID, *p.RoomID, p.Date, p.StartTime, p.EndTime, excludeID)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrRoomBusy
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

// SessionTopic returns the recorded topic for a group's session on a date (empty if none).
func (s *Service) SessionTopic(ctx context.Context, orgID, groupID uuid.UUID, date time.Time) (string, error) {
	l, found, err := s.repo.FindByGroupDate(ctx, orgID, groupID, date)
	if err != nil || !found {
		return "", err
	}
	return l.Topic, nil
}

// SaveSessionTopic records "what was taught" for a group on a date. It upserts a single lesson
// row per (group, date): updating the existing one or creating a new done session. Called when a
// teacher takes attendance and notes the topic.
func (s *Service) SaveSessionTopic(ctx context.Context, orgID, groupID uuid.UUID, date time.Time, topic string) error {
	if date.IsZero() {
		return ErrValidation
	}
	topic = strings.TrimSpace(topic)
	existing, found, err := s.repo.FindByGroupDate(ctx, orgID, groupID, date)
	if err != nil {
		return err
	}
	if found {
		_, uerr := s.repo.Update(ctx, orgID, existing.ID, repo.LessonParams{
			GroupID:   &groupID,
			TeacherID: existing.TeacherID,
			Date:      date,
			StartTime: existing.StartTime,
			EndTime:   existing.EndTime,
			Room:      existing.Room,
			RoomID:    existing.RoomID,
			Topic:     topic,
			Status:    "done",
		})
		return uerr
	}
	_, cerr := s.repo.Create(ctx, orgID, repo.LessonParams{
		GroupID: &groupID,
		Date:    date,
		Topic:   topic,
		Status:  "done",
	})
	return cerr
}

func (s *Service) normalize(in Input) (repo.LessonParams, error) {
	if in.Date.IsZero() {
		return repo.LessonParams{}, ErrValidation
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "scheduled"
	}
	if !entity.LessonStatuses[status] {
		return repo.LessonParams{}, ErrValidation
	}
	return repo.LessonParams{
		GroupID:   in.GroupID,
		TeacherID: in.TeacherID,
		Date:      in.Date,
		StartTime: strings.TrimSpace(in.StartTime),
		EndTime:   strings.TrimSpace(in.EndTime),
		Room:      strings.TrimSpace(in.Room),
		RoomID:    in.RoomID,
		Topic:     strings.TrimSpace(in.Topic),
		Status:    status,
	}, nil
}
