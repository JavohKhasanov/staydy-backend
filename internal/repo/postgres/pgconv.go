package postgres

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/repo/sqlc"
)

// --- pgtype <-> Go conversions ---

func textToString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

// uuidPtr makes a nullable pgtype.UUID — invalid (SQL NULL) when id is the zero uuid.
func uuidPtr(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

// pgUUID reads a pgtype.UUID back to a uuid.UUID (uuid.Nil when SQL NULL).
func pgUUID(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// nullableUUID makes a pgtype.UUID from an optional id (SQL NULL when nil).
func nullableUUID(p *uuid.UUID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

// uuidToPtr reads a nullable pgtype.UUID to *uuid.UUID (nil when SQL NULL).
func uuidToPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	v := uuid.UUID(p.Bytes)
	return &v
}

func textVal(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func tsVal(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func tsToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func tsPtrVal(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func dateToPtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	v := d.Time
	return &v
}

func dateVal(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func int8ToPtr(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func pgInt8(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// --- sqlc row -> entity mappers ---

func mapOrg(o sqlc.Organization) entity.Organization {
	return entity.Organization{
		ID:            o.ID,
		Name:          o.Name,
		Slug:          o.Slug,
		Plan:          o.Plan,
		Status:        o.Status,
		TrialEndsAt:   tsToPtr(o.TrialEndsAt),
		BillingStatus: o.BillingStatus,
		CreatedAt:     o.CreatedAt.Time,
		UpdatedAt:     o.UpdatedAt.Time,
	}
}

func mapBranch(b sqlc.Branch) entity.Branch {
	return entity.Branch{
		ID:        b.ID,
		OrgID:     b.OrgID,
		Name:      b.Name,
		Address:   b.Address,
		Phone:     b.Phone,
		IsActive:  b.IsActive,
		CreatedAt: b.CreatedAt.Time,
	}
}

func mapRoom(r sqlc.Room) entity.Room {
	return entity.Room{
		ID:        r.ID,
		OrgID:     r.OrgID,
		BranchID:  uuidToPtr(r.BranchID),
		Name:      r.Name,
		Capacity:  int(r.Capacity),
		CreatedAt: r.CreatedAt.Time,
	}
}

func mapSalaryRule(r sqlc.SalaryRule) entity.SalaryRule {
	return entity.SalaryRule{
		ID:        r.ID,
		OrgID:     r.OrgID,
		TeacherID: r.TeacherID,
		Kind:      r.Kind,
		Rate:      r.Rate,
	}
}

func mapSalarySlip(s sqlc.SalarySlip) entity.SalarySlip {
	return entity.SalarySlip{
		ID:          s.ID,
		OrgID:       s.OrgID,
		TeacherID:   s.TeacherID,
		PeriodStart: s.PeriodStart.Time,
		PeriodEnd:   s.PeriodEnd.Time,
		Gross:       s.Gross,
		Bonus:       s.Bonus,
		Deduction:   s.Deduction,
		Net:         s.Net,
		Status:      s.Status,
		Note:        s.Note,
		PaidAt:      tsToPtr(s.PaidAt),
		CreatedAt:   s.CreatedAt.Time,
	}
}

func mapSalarySlipRow(s sqlc.ListSalarySlipsRow) entity.SalarySlip {
	return entity.SalarySlip{
		ID:          s.ID,
		OrgID:       s.OrgID,
		TeacherID:   s.TeacherID,
		TeacherName: s.TeacherName,
		PeriodStart: s.PeriodStart.Time,
		PeriodEnd:   s.PeriodEnd.Time,
		Gross:       s.Gross,
		Bonus:       s.Bonus,
		Deduction:   s.Deduction,
		Net:         s.Net,
		Status:      s.Status,
		Note:        s.Note,
		PaidAt:      tsToPtr(s.PaidAt),
		CreatedAt:   s.CreatedAt.Time,
	}
}

func mapSignupRequest(s sqlc.SignupRequest) entity.SignupRequest {
	return entity.SignupRequest{
		ID:          s.ID,
		CenterName:  s.CenterName,
		ContactName: s.ContactName,
		Phone:       s.Phone,
		Email:       s.Email,
		Plan:        s.Plan,
		Message:     s.Message,
		Status:      s.Status,
		CreatedAt:   s.CreatedAt.Time,
	}
}

func mapUser(u sqlc.User) entity.User {
	return entity.User{
		ID:           u.ID,
		OrgID:        u.OrgID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Role:         entity.UserRole(u.Role),
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

func mapGroup(g sqlc.Group) entity.Group {
	return entity.Group{
		ID:           g.ID,
		OrgID:        g.OrgID,
		Name:         g.Name,
		TeacherID:    uuidToPtr(g.TeacherID),
		CourseID:     uuidToPtr(g.CourseID),
		BranchID:     uuidToPtr(g.BranchID),
		Direction:    textToString(g.Direction),
		ScheduleDays: textToString(g.ScheduleDays),
		Capacity:     int(g.Capacity),
		CreatedAt:    g.CreatedAt.Time,
		UpdatedAt:    g.UpdatedAt.Time,
	}
}

func mapStudent(s sqlc.Student) entity.Student {
	return entity.Student{
		ID:               s.ID,
		OrgID:            s.OrgID,
		Name:             s.Name,
		Phone:            textToString(s.Phone),
		TelegramID:       textToString(s.TelegramID),
		TelegramChatID:   int8ToPtr(s.TelegramChatID),
		CourseName:       textToString(s.CourseName),
		GroupName:        textToString(s.GroupName),
		GroupID:          uuidToPtr(s.GroupID),
		MentorName:       textToString(s.MentorName),
		StartDate:        dateToPtr(s.StartDate),
		OnboardingGoal:   textToString(s.OnboardingGoal),
		SixMonthTarget:   textToString(s.SixMonthTarget),
		WeeklyStudyHours: textToString(s.WeeklyStudyHours),
		ConfidenceLevel:  int(s.ConfidenceLevel),
		RiskScore:        int(s.RiskScore),
		RiskTier:         s.RiskTier,
		Email:            textToString(s.Email),
		BirthDate:        dateToPtr(s.BirthDate),
		Gender:           s.Gender,
		SecondPhone:      textToString(s.SecondPhone),
		Address:          textToString(s.Address),
		ParentName:       textToString(s.ParentName),
		ParentPhone:      textToString(s.ParentPhone),
		StudentCode:      textToString(s.StudentCode),
		Status:           s.Status,
		MentorID:         uuidToPtr(s.MentorID),
		BranchID:         uuidToPtr(s.BranchID),
		CreatedAt:        s.CreatedAt.Time,
		UpdatedAt:        s.UpdatedAt.Time,
	}
}

func mapSurvey(s sqlc.Survey) entity.Survey {
	return entity.Survey{
		ID:              s.ID,
		OrgID:           s.OrgID,
		StudentID:       s.StudentID,
		WeekNumber:      int(s.WeekNumber),
		MotivationScore: int(s.MotivationScore),
		ProgressScore:   int(s.ProgressScore),
		BiggestObstacle: textToString(s.BiggestObstacle),
		Comment:         textToString(s.Comment),
		SubmittedAt:     s.SubmittedAt.Time,
	}
}

func mapAttendance(a sqlc.AttendanceRecord) entity.AttendanceRecord {
	return entity.AttendanceRecord{
		ID:        a.ID,
		OrgID:     a.OrgID,
		StudentID: a.StudentID,
		Date:      a.Date.Time,
		IsPresent: a.IsPresent,
		Status:    a.Status,
		CreatedAt: a.CreatedAt.Time,
	}
}

func mapHomework(h sqlc.HomeworkRecord) entity.HomeworkRecord {
	return entity.HomeworkRecord{
		ID:        h.ID,
		OrgID:     h.OrgID,
		StudentID: h.StudentID,
		Date:      h.Date.Time,
		IsDone:    h.IsDone,
		CreatedAt: h.CreatedAt.Time,
	}
}

func mapTask(t sqlc.InterventionTask) entity.InterventionTask {
	return entity.InterventionTask{
		ID:                t.ID,
		OrgID:             t.OrgID,
		StudentID:         t.StudentID,
		Reasons:           t.Reasons,
		Status:            t.Status,
		ResolutionComment: textToString(t.ResolutionComment),
		CreatedAt:         t.CreatedAt.Time,
		ResolvedAt:        tsToPtr(t.ResolvedAt),
	}
}

func mapTaskRow(t sqlc.ListInterventionTasksRow) entity.InterventionTask {
	return entity.InterventionTask{
		ID:                t.ID,
		OrgID:             t.OrgID,
		StudentID:         t.StudentID,
		StudentName:       t.StudentName,
		Reasons:           t.Reasons,
		Status:            t.Status,
		ResolutionComment: textToString(t.ResolutionComment),
		CreatedAt:         t.CreatedAt.Time,
		ResolvedAt:        tsToPtr(t.ResolvedAt),
	}
}

func mapNote(n sqlc.Note) entity.Note {
	return entity.Note{
		ID:        n.ID,
		OrgID:     n.OrgID,
		StudentID: n.StudentID,
		Author:    n.Author,
		Text:      n.Text,
		CreatedAt: n.CreatedAt.Time,
	}
}

func mapSession(s sqlc.RefreshSession) entity.RefreshSession {
	return entity.RefreshSession{
		ID:        s.ID,
		UserID:    s.UserID,
		OrgID:     s.OrgID,
		TokenHash: s.TokenHash,
		UserAgent: textToString(s.UserAgent),
		IP:        textToString(s.Ip),
		ExpiresAt: s.ExpiresAt.Time,
		RevokedAt: tsToPtr(s.RevokedAt),
		CreatedAt: s.CreatedAt.Time,
	}
}
