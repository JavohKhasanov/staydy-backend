package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

// --- DTOs ---

type createStudentRequest struct {
	Name             string `json:"name" minLength:"1" maxLength:"255" example:"Ali Valiyev"`
	Phone            string `json:"phone,omitempty" maxLength:"32"`
	TelegramID       string `json:"telegramId,omitempty" maxLength:"64"`
	CourseName       string `json:"courseName,omitempty" maxLength:"255"`
	GroupName        string `json:"groupName,omitempty" maxLength:"255"`
	MentorName       string `json:"mentorName,omitempty" maxLength:"255"`
	StartDate        string `json:"startDate,omitempty" format:"date" doc:"YYYY-MM-DD"`
	OnboardingGoal   string `json:"onboardingGoal,omitempty" maxLength:"255"`
	SixMonthTarget   string `json:"sixMonthTarget,omitempty" maxLength:"2000"`
	WeeklyStudyHours string `json:"weeklyStudyHours,omitempty" maxLength:"64"`
	ConfidenceLevel  int    `json:"confidenceLevel" minimum:"1" maximum:"10" example:"8"`
	Email            string `json:"email,omitempty" maxLength:"255"`
	BirthDate        string `json:"birthDate,omitempty" format:"date" doc:"YYYY-MM-DD"`
	Gender           string `json:"gender,omitempty" maxLength:"20"`
	SecondPhone      string `json:"secondPhone,omitempty" maxLength:"32"`
	Address          string `json:"address,omitempty" maxLength:"500"`
	ParentName       string `json:"parentName,omitempty" maxLength:"255"`
	ParentPhone      string `json:"parentPhone,omitempty" maxLength:"32"`
	StudentCode      string `json:"studentCode,omitempty" maxLength:"64"`
	Status           string `json:"status,omitempty" enum:"active,inactive,graduated,lead,dropped"`
	MentorID         string `json:"mentorId,omitempty"`
	BranchID         string `json:"branchId,omitempty"`
}

// updateStudentRequest is the editable profile (group + risk are managed by their own endpoints).
type updateStudentRequest struct {
	Name             string `json:"name" minLength:"1" maxLength:"255"`
	Phone            string `json:"phone,omitempty" maxLength:"32"`
	StartDate        string `json:"startDate,omitempty" format:"date" doc:"YYYY-MM-DD"`
	OnboardingGoal   string `json:"onboardingGoal,omitempty" maxLength:"255"`
	SixMonthTarget   string `json:"sixMonthTarget,omitempty" maxLength:"2000"`
	WeeklyStudyHours string `json:"weeklyStudyHours,omitempty" maxLength:"64"`
	ConfidenceLevel  int    `json:"confidenceLevel" minimum:"1" maximum:"10"`
	Email            string `json:"email,omitempty" maxLength:"255"`
	BirthDate        string `json:"birthDate,omitempty" format:"date" doc:"YYYY-MM-DD"`
	Gender           string `json:"gender,omitempty" maxLength:"20"`
	SecondPhone      string `json:"secondPhone,omitempty" maxLength:"32"`
	Address          string `json:"address,omitempty" maxLength:"500"`
	ParentName       string `json:"parentName,omitempty" maxLength:"255"`
	ParentPhone      string `json:"parentPhone,omitempty" maxLength:"32"`
	StudentCode      string `json:"studentCode,omitempty" maxLength:"64"`
	Status           string `json:"status,omitempty" enum:"active,inactive,graduated,lead,dropped"`
	MentorID         string `json:"mentorId,omitempty"`
	BranchID         string `json:"branchId,omitempty"`
}

type addNoteRequest struct {
	Text string `json:"text" minLength:"1" maxLength:"2000"`
}

type studentResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Phone            string `json:"phone,omitempty"`
	TelegramID       string `json:"telegramId,omitempty"`
	CourseName       string `json:"courseName,omitempty"`
	GroupName        string `json:"groupName,omitempty"`
	GroupID          string `json:"groupId,omitempty"`
	MentorName       string `json:"mentorName,omitempty"`
	StartDate        string `json:"startDate,omitempty"`
	OnboardingGoal   string `json:"onboardingGoal,omitempty"`
	SixMonthTarget   string `json:"sixMonthTarget,omitempty"`
	WeeklyStudyHours string `json:"weeklyStudyHours,omitempty"`
	ConfidenceLevel  int    `json:"confidenceLevel"`
	RiskScore        int    `json:"riskScore"`
	RiskTier         string `json:"riskTier"`
	Email            string `json:"email,omitempty"`
	BirthDate        string `json:"birthDate,omitempty"`
	Gender           string `json:"gender,omitempty"`
	SecondPhone      string `json:"secondPhone,omitempty"`
	Address          string `json:"address,omitempty"`
	ParentName       string `json:"parentName,omitempty"`
	ParentPhone      string `json:"parentPhone,omitempty"`
	StudentCode      string `json:"studentCode,omitempty"`
	Status           string `json:"status,omitempty"`
	MentorID         string `json:"mentorId,omitempty"`
	BranchID         string `json:"branchId,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

type noteResponse struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"createdAt"`
}

type riskFactorResponse struct {
	Label  string `json:"label"`
	Points int    `json:"points"`
}

type submitSurveyRequest struct {
	WeekNumber      int    `json:"weekNumber" minimum:"1" example:"4"`
	MotivationScore int    `json:"motivationScore" minimum:"1" maximum:"5" example:"2"`
	ProgressScore   int    `json:"progressScore" minimum:"1" maximum:"5" example:"2"`
	BiggestObstacle string `json:"biggestObstacle,omitempty" maxLength:"255"`
	Comment         string `json:"comment,omitempty" maxLength:"2000"`
}

type recordAttendanceRequest struct {
	Date    string `json:"date" format:"date" minLength:"10" doc:"YYYY-MM-DD"`
	GroupID string `json:"groupId,omitempty" format:"uuid" doc:"which group's session (multi-group students)"`
	Status  string `json:"status,omitempty" enum:"present,absent,late,excused" doc:"attendance status (default present)"`
	// IsPresent is the legacy boolean; kept for backward compatibility. Ignored when Status is set.
	IsPresent *bool `json:"isPresent,omitempty"`
}

type surveyResponse struct {
	ID              string `json:"id"`
	WeekNumber      int    `json:"weekNumber"`
	MotivationScore int    `json:"motivationScore"`
	ProgressScore   int    `json:"progressScore"`
	BiggestObstacle string `json:"biggestObstacle,omitempty"`
	Comment         string `json:"comment,omitempty"`
	SubmittedAt     string `json:"submittedAt"`
}

type attendanceResponse struct {
	ID        string `json:"id"`
	Date      string `json:"date"`
	IsPresent bool   `json:"isPresent"`
	Status    string `json:"status"`
}

type recordHomeworkRequest struct {
	Date   string `json:"date" format:"date" minLength:"10" doc:"YYYY-MM-DD"`
	IsDone bool   `json:"isDone"`
}

type homeworkResponse struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	IsDone bool   `json:"isDone"`
}

type studentDetailResponse struct {
	Student     studentResponse      `json:"student"`
	Notes       []noteResponse       `json:"notes"`
	Surveys     []surveyResponse     `json:"surveys"`
	Attendance  []attendanceResponse `json:"attendance"`
	Homework    []homeworkResponse   `json:"homework"`
	RiskFactors []riskFactorResponse `json:"riskFactors"`
}

// --- inputs / outputs ---

type listStudentsInput struct{}
type listStudentsOutput struct{ Body []studentResponse }
type createStudentInput struct{ Body createStudentRequest }
type updateStudentInput struct {
	ID   string `path:"id" format:"uuid"`
	Body updateStudentRequest
}
type studentOutput struct{ Body studentResponse }
type studentIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type studentDetailOutput struct{ Body studentDetailResponse }
type addNoteInput struct {
	ID   string `path:"id" format:"uuid"`
	Body addNoteRequest
}
type noteOutput struct{ Body noteResponse }
type submitSurveyInput struct {
	ID   string `path:"id" format:"uuid"`
	Body submitSurveyRequest
}
type surveyOutput struct{ Body surveyResponse }
type recordAttendanceInput struct {
	ID   string `path:"id" format:"uuid"`
	Body recordAttendanceRequest
}
type attendanceOutput struct{ Body attendanceResponse }
type recordHomeworkInput struct {
	ID   string `path:"id" format:"uuid"`
	Body recordHomeworkRequest
}
type homeworkOutput struct{ Body homeworkResponse }

// registerStudents mounts the tenant-scoped student operations on the protected group.
func registerStudents(api huma.API, svc *studentusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-list",
		Method:      http.MethodGet,
		Path:        "/students",
		Summary:     "List students in the center (highest risk first)",
		Tags:        []string{"students"},
		Errors:      []int{http.StatusInternalServerError},
	}), func(ctx context.Context, _ *listStudentsInput) (*listStudentsOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		students, err := svc.List(ctx, p.OrgID)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		out := &listStudentsOutput{Body: make([]studentResponse, 0, len(students))}
		for _, st := range students {
			out.Body = append(out.Body, toStudentResponse(st))
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-create",
		Method:        http.MethodPost,
		Path:          "/students",
		Summary:       "Onboard a new student (risk scored on create)",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createStudentInput) (*studentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		startDate, perr := parseOptDate(in.Body.StartDate)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("startDate must be YYYY-MM-DD")
		}
		birthDate, berr := parseOptDate(in.Body.BirthDate)
		if berr != nil {
			return nil, huma.Error422UnprocessableEntity("birthDate must be YYYY-MM-DD")
		}
		mentorID, merr := parseOptUUID(in.Body.MentorID)
		if merr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid mentorId")
		}
		branchID, brerr := parseOptUUID(in.Body.BranchID)
		if brerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		st, err := svc.Create(ctx, p.OrgID, studentusecase.CreateParams{
			Name:             in.Body.Name,
			Phone:            in.Body.Phone,
			TelegramID:       in.Body.TelegramID,
			CourseName:       in.Body.CourseName,
			GroupName:        in.Body.GroupName,
			MentorName:       in.Body.MentorName,
			StartDate:        startDate,
			OnboardingGoal:   in.Body.OnboardingGoal,
			SixMonthTarget:   in.Body.SixMonthTarget,
			WeeklyStudyHours: in.Body.WeeklyStudyHours,
			ConfidenceLevel:  in.Body.ConfidenceLevel,
			Email:            in.Body.Email,
			BirthDate:        birthDate,
			Gender:           in.Body.Gender,
			SecondPhone:      in.Body.SecondPhone,
			Address:          in.Body.Address,
			ParentName:       in.Body.ParentName,
			ParentPhone:      in.Body.ParentPhone,
			StudentCode:      in.Body.StudentCode,
			Status:           in.Body.Status,
			MentorID:         mentorID,
			BranchID:         branchID,
		})
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &studentOutput{Body: toStudentResponse(st)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-update",
		Method:      http.MethodPut,
		Path:        "/students/{id}",
		Summary:     "Edit a student's profile (group + risk have their own endpoints)",
		Tags:        []string{"students"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateStudentInput) (*studentOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		startDate, perr := parseOptDate(in.Body.StartDate)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("startDate must be YYYY-MM-DD")
		}
		birthDate, berr := parseOptDate(in.Body.BirthDate)
		if berr != nil {
			return nil, huma.Error422UnprocessableEntity("birthDate must be YYYY-MM-DD")
		}
		mentorID, merr := parseOptUUID(in.Body.MentorID)
		if merr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid mentorId")
		}
		branchID, brerr := parseOptUUID(in.Body.BranchID)
		if brerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid branchId")
		}
		st, err := svc.Update(ctx, p.OrgID, id, studentusecase.UpdateParams{
			Name:             in.Body.Name,
			Phone:            in.Body.Phone,
			StartDate:        startDate,
			OnboardingGoal:   in.Body.OnboardingGoal,
			SixMonthTarget:   in.Body.SixMonthTarget,
			WeeklyStudyHours: in.Body.WeeklyStudyHours,
			ConfidenceLevel:  in.Body.ConfidenceLevel,
			Email:            in.Body.Email,
			BirthDate:        birthDate,
			Gender:           in.Body.Gender,
			SecondPhone:      in.Body.SecondPhone,
			Address:          in.Body.Address,
			ParentName:       in.Body.ParentName,
			ParentPhone:      in.Body.ParentPhone,
			StudentCode:      in.Body.StudentCode,
			Status:           in.Body.Status,
			MentorID:         mentorID,
			BranchID:         branchID,
		})
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &studentOutput{Body: toStudentResponse(st)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-delete",
		Method:        http.MethodDelete,
		Path:          "/students/{id}",
		Summary:       "Delete a student and all their records (cascade)",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentIDInput) (*struct{}, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		if err := svc.Delete(ctx, p.OrgID, id); err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return nil, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "students-get",
		Method:      http.MethodGet,
		Path:        "/students/{id}",
		Summary:     "Get a student with notes + risk-factor breakdown",
		Tags:        []string{"students"},
		Errors:      []int{http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *studentIDInput) (*studentDetailOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		detail, err := svc.Get(ctx, p.OrgID, id)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		resp := studentDetailResponse{
			Student:     toStudentResponse(detail.Student),
			Notes:       make([]noteResponse, 0, len(detail.Notes)),
			Surveys:     make([]surveyResponse, 0, len(detail.Surveys)),
			Attendance:  make([]attendanceResponse, 0, len(detail.Attendance)),
			Homework:    make([]homeworkResponse, 0, len(detail.Homework)),
			RiskFactors: make([]riskFactorResponse, 0, len(detail.Factors)),
		}
		for _, n := range detail.Notes {
			resp.Notes = append(resp.Notes, toNoteResponse(n))
		}
		for _, sv := range detail.Surveys {
			resp.Surveys = append(resp.Surveys, toSurveyResponse(sv))
		}
		for _, a := range detail.Attendance {
			resp.Attendance = append(resp.Attendance, toAttendanceResponse(a))
		}
		for _, h := range detail.Homework {
			resp.Homework = append(resp.Homework, toHomeworkResponse(h))
		}
		for _, f := range detail.Factors {
			resp.RiskFactors = append(resp.RiskFactors, riskFactorResponse{Label: f.Label, Points: f.Points})
		}
		return &studentDetailOutput{Body: resp}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-submit-survey",
		Method:        http.MethodPost,
		Path:          "/students/{id}/surveys",
		Summary:       "Submit a weekly check-in (recomputes risk; may open a task)",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *submitSurveyInput) (*surveyOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		sv, err := svc.SubmitSurvey(ctx, p.OrgID, id, studentusecase.SubmitSurveyParams{
			WeekNumber:      in.Body.WeekNumber,
			MotivationScore: in.Body.MotivationScore,
			ProgressScore:   in.Body.ProgressScore,
			BiggestObstacle: in.Body.BiggestObstacle,
			Comment:         in.Body.Comment,
		})
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &surveyOutput{Body: toSurveyResponse(sv)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-record-attendance",
		Method:        http.MethodPost,
		Path:          "/students/{id}/attendance",
		Summary:       "Record one lesson's attendance (recomputes risk; may open a task)",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordAttendanceInput) (*attendanceOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		date, err := time.Parse("2006-01-02", in.Body.Date)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD")
		}
		gid, gerr := parseOptUUID(in.Body.GroupID)
		if gerr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid groupId")
		}
		rec, err := svc.RecordAttendance(ctx, p.OrgID, id, gid, date, attendanceStatus(in.Body))
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &attendanceOutput{Body: toAttendanceResponse(rec)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-record-homework",
		Method:        http.MethodPost,
		Path:          "/students/{id}/homework",
		Summary:       "Record one lesson's homework completion (recomputes risk; may open a task)",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *recordHomeworkInput) (*homeworkOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		date, err := time.Parse("2006-01-02", in.Body.Date)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("date must be YYYY-MM-DD")
		}
		rec, err := svc.RecordHomework(ctx, p.OrgID, id, date, in.Body.IsDone)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &homeworkOutput{Body: toHomeworkResponse(rec)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "students-add-note",
		Method:        http.MethodPost,
		Path:          "/students/{id}/notes",
		Summary:       "Add a mentor note to a student",
		Tags:          []string{"students"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusInternalServerError},
	}), func(ctx context.Context, in *addNoteInput) (*noteOutput, error) {
		p, err := principal(ctx)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("invalid student id")
		}
		note, err := svc.AddNote(ctx, p.OrgID, id, p.FullName, in.Body.Text)
		if err != nil {
			return nil, mapStudentError(LangFromContext(ctx), err, log)
		}
		return &noteOutput{Body: toNoteResponse(note)}, nil
	})
}

// principal extracts the authenticated identity, or returns 401 if absent (defensive —
// these routes sit behind RequireAuth).
func principal(ctx context.Context) (entity.Principal, error) {
	p, ok := PrincipalFromContext(ctx)
	if !ok || p.OrgID == uuid.Nil {
		// A token without an org claim (e.g. a future platform/super-admin token) must
		// never enter a tenant-scoped transaction.
		return entity.Principal{}, huma.Error401Unauthorized("missing or non-tenant principal")
	}
	return p, nil
}

func toStudentResponse(s entity.Student) studentResponse {
	r := studentResponse{
		ID:               s.ID.String(),
		Name:             s.Name,
		Phone:            s.Phone,
		TelegramID:       s.TelegramID,
		CourseName:       s.CourseName,
		GroupName:        s.GroupName,
		MentorName:       s.MentorName,
		OnboardingGoal:   s.OnboardingGoal,
		SixMonthTarget:   s.SixMonthTarget,
		WeeklyStudyHours: s.WeeklyStudyHours,
		ConfidenceLevel:  s.ConfidenceLevel,
		RiskScore:        s.RiskScore,
		RiskTier:         s.RiskTier,
		Email:            s.Email,
		Gender:           s.Gender,
		SecondPhone:      s.SecondPhone,
		Address:          s.Address,
		ParentName:       s.ParentName,
		ParentPhone:      s.ParentPhone,
		StudentCode:      s.StudentCode,
		Status:           s.Status,
		CreatedAt:        s.CreatedAt.UTC().Format(time.RFC3339),
	}
	if s.StartDate != nil {
		r.StartDate = s.StartDate.Format("2006-01-02")
	}
	if s.BirthDate != nil {
		r.BirthDate = s.BirthDate.Format("2006-01-02")
	}
	if s.GroupID != nil {
		r.GroupID = s.GroupID.String()
	}
	if s.MentorID != nil {
		r.MentorID = s.MentorID.String()
	}
	if s.BranchID != nil {
		r.BranchID = s.BranchID.String()
	}
	return r
}

func toNoteResponse(n entity.Note) noteResponse {
	return noteResponse{
		ID:        n.ID.String(),
		Author:    n.Author,
		Text:      n.Text,
		CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toSurveyResponse(s entity.Survey) surveyResponse {
	return surveyResponse{
		ID:              s.ID.String(),
		WeekNumber:      s.WeekNumber,
		MotivationScore: s.MotivationScore,
		ProgressScore:   s.ProgressScore,
		BiggestObstacle: s.BiggestObstacle,
		Comment:         s.Comment,
		SubmittedAt:     s.SubmittedAt.UTC().Format(time.RFC3339),
	}
}

func toAttendanceResponse(a entity.AttendanceRecord) attendanceResponse {
	return attendanceResponse{
		ID:        a.ID.String(),
		Date:      a.Date.Format("2006-01-02"),
		IsPresent: a.IsPresent,
		Status:    a.Status,
	}
}

// attendanceStatus resolves the attendance status from the request: the new Status field wins;
// otherwise it falls back to the legacy IsPresent boolean. Empty defaults to present.
func attendanceStatus(b recordAttendanceRequest) string {
	if b.Status != "" {
		return b.Status
	}
	if b.IsPresent != nil && !*b.IsPresent {
		return entity.AttendanceAbsent
	}
	return entity.AttendancePresent
}

func toHomeworkResponse(h entity.HomeworkRecord) homeworkResponse {
	return homeworkResponse{
		ID:     h.ID.String(),
		Date:   h.Date.Format("2006-01-02"),
		IsDone: h.IsDone,
	}
}

var msgStudentNotFound = i18n.Message{UZ: "Talaba topilmadi.", RU: "Студент не найден.", EN: "Student not found."}

func mapStudentError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, studentusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, studentusecase.ErrNotFound):
		return huma.Error404NotFound(msgStudentNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("students: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}
