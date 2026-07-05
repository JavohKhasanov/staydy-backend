package http

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/i18n"
	"github.com/student-success/backend/internal/repo"
	authusecase "github.com/student-success/backend/internal/usecase/auth"
	superadminusecase "github.com/student-success/backend/internal/usecase/superadmin"
)

// --- response shapes (match SUPERADMIN_LOVABLE_PROMPT.md) ---

type centerSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Plan          string `json:"plan"`
	Status        string `json:"status"`
	BillingStatus string `json:"billingStatus"`         // trial | active (paid) | expired
	TrialEndsAt   string `json:"trialEndsAt,omitempty"` // ISO; end of the free month
	TrialDaysLeft int    `json:"trialDaysLeft"`         // whole days left (negative if past)
	StudentCount  int    `json:"studentCount"`
	UserCount     int    `json:"userCount"`
	AtRiskCount   int    `json:"atRiskCount"`
	CreatedAt     string `json:"createdAt"`
}

// centerDetail is a flat struct (NOT an embed of centerSummary): Huma's schema reflection does
// not promote anonymous embedded struct fields, so embedding would drop them from the response.
type centerDetail struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Plan         string `json:"plan"`
	Status       string `json:"status"`
	StudentCount  int    `json:"studentCount"`
	UserCount     int    `json:"userCount"`
	AtRiskCount   int    `json:"atRiskCount"`
	CreatedAt     string `json:"createdAt"`
	BillingStatus string `json:"billingStatus"`
	TrialEndsAt   string `json:"trialEndsAt,omitempty"`
	TrialDaysLeft int    `json:"trialDaysLeft"`
	GreenCount    int    `json:"greenCount"`
	YellowCount   int    `json:"yellowCount"`
	RedCount      int    `json:"redCount"`
}

type centerAdminResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
}

// --- admin login (public) ---

type adminLoginRequest struct {
	Email    string `json:"email" format:"email"`
	Password string `json:"password" minLength:"1"`
}
type adminLoginInput struct {
	UserAgent string `header:"User-Agent"`
	Body      adminLoginRequest
}
type adminAuthOutput struct{ Body authResponse }

func registerAdminAuth(api huma.API, auth *authusecase.Service, log zerolog.Logger) {
	Register(api, huma.Operation{
		OperationID: "admin-login",
		Method:      http.MethodPost,
		Path:        "/admin/login",
		Summary:     "Superadmin login (platform owner; no center slug)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden},
	}, func(ctx context.Context, in *adminLoginInput) (*adminAuthOutput, error) {
		tokens, err := auth.Login(ctx, authusecase.LoginParams{
			Slug:     superadminusecase.PlatformSlug,
			Email:    in.Body.Email,
			Password: in.Body.Password,
		}, authusecase.DeviceMetadata{UserAgent: in.UserAgent})
		if err != nil {
			return nil, mapAuthError(LangFromContext(ctx), err, log)
		}
		if tokens.User.Role != entity.RoleSuperAdmin {
			return nil, huma.Error401Unauthorized("not a superadmin account")
		}
		return &adminAuthOutput{Body: toAuthResponse(tokens)}, nil
	})
}

// --- center management (super_admin gated) ---

type createCenterRequest struct {
	OrganizationName string `json:"organizationName" minLength:"1" maxLength:"255"`
	Slug             string `json:"slug" minLength:"2" maxLength:"63" pattern:"^[a-z0-9-]+$"`
	Plan             string `json:"plan,omitempty" doc:"trial | basic | pro (default trial)"`
	AdminFullName    string `json:"adminFullName" minLength:"1" maxLength:"255"`
	AdminEmail       string `json:"adminEmail" format:"email"`
	AdminPassword    string `json:"adminPassword" minLength:"8" maxLength:"128"`
}
type updateCenterRequest struct {
	Plan   string `json:"plan,omitempty" enum:"trial,basic,pro"`
	Status string `json:"status,omitempty" enum:"active,suspended"`
}
type setBillingRequest struct {
	Plan          string `json:"plan,omitempty" enum:"trial,basic,pro,internal"`
	BillingStatus string `json:"billingStatus,omitempty" enum:"trial,active,expired"`
	TrialEndsAt   string `json:"trialEndsAt,omitempty" doc:"YYYY-MM-DD — sets/extends the free-trial end"`
}
type planCount struct {
	Plan  string `json:"plan"`
	Count int    `json:"count"`
}
type tierCounts struct {
	Green  int `json:"green"`
	Yellow int `json:"yellow"`
	Red    int `json:"red"`
}
type metricsResponse struct {
	TotalCenters     int             `json:"totalCenters"`
	ActiveCenters    int             `json:"activeCenters"`
	SuspendedCenters int             `json:"suspendedCenters"`
	TotalStudents    int             `json:"totalStudents"`
	TotalAtRisk      int             `json:"totalAtRisk"`
	CentersByPlan    []planCount     `json:"centersByPlan"`
	StudentsByTier   tierCounts      `json:"studentsByTier"`
	RecentCenters    []centerSummary `json:"recentCenters"`
}

type centersListInput struct {
	Query  string `query:"query"`
	Plan   string `query:"plan"`
	Status string `query:"status"`
}
type centersListOutput struct{ Body []centerSummary }
type centerIDInput struct {
	ID string `path:"id" format:"uuid"`
}
type updateCenterInput struct {
	ID   string `path:"id" format:"uuid"`
	Body updateCenterRequest
}
type setBillingInput struct {
	ID   string `path:"id" format:"uuid"`
	Body setBillingRequest
}
type createCenterInput struct{ Body createCenterRequest }
type centerDetailOutput struct{ Body centerDetail }
type centerSummaryOutput struct{ Body centerSummary }
type metricsOutput struct{ Body metricsResponse }
type createCenterOutput struct {
	Body struct {
		Center centerSummary       `json:"center"`
		Admin  centerAdminResponse `json:"admin"`
	}
}

func registerAdmin(api huma.API, super *superadminusecase.Service, log zerolog.Logger) {
	Register(api, BearerOperation(huma.Operation{
		OperationID: "admin-centers-list",
		Method:      http.MethodGet,
		Path:        "/admin/centers",
		Summary:     "List centers (filter by query/plan/status)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *centersListInput) (*centersListOutput, error) {
		centers, err := super.ListCenters(ctx)
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		out := &centersListOutput{Body: make([]centerSummary, 0, len(centers))}
		for _, c := range centers {
			if centerMatches(c, in.Query, in.Plan, in.Status) {
				out.Body = append(out.Body, toCenterSummary(c))
			}
		}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "admin-centers-create",
		Method:        http.MethodPost,
		Path:          "/admin/centers",
		Summary:       "Create a center and its first admin",
		Tags:          []string{"admin"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusConflict, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *createCenterInput) (*createCenterOutput, error) {
		org, user, err := super.CreateCenter(ctx, superadminusecase.CreateCenterInput{
			OrganizationName: in.Body.OrganizationName,
			Slug:             in.Body.Slug,
			Plan:             in.Body.Plan,
			AdminFullName:    in.Body.AdminFullName,
			AdminEmail:       in.Body.AdminEmail,
			AdminPassword:    in.Body.AdminPassword,
		})
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		out := &createCenterOutput{}
		out.Body.Center = toCenterSummary(repo.CenterStats{Org: org})
		out.Body.Admin = centerAdminResponse{ID: user.ID.String(), Email: user.Email, FullName: user.FullName, Role: string(user.Role)}
		return out, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "admin-centers-get",
		Method:      http.MethodGet,
		Path:        "/admin/centers/{id}",
		Summary:     "Get a center with its risk breakdown",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *centerIDInput) (*centerDetailOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid center id")
		}
		cs, err := super.GetCenter(ctx, id)
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		return &centerDetailOutput{Body: toCenterDetail(cs)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "admin-centers-update",
		Method:      http.MethodPatch,
		Path:        "/admin/centers/{id}",
		Summary:     "Update a center's plan and/or status (suspend/activate)",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *updateCenterInput) (*centerSummaryOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid center id")
		}
		upd := superadminusecase.UpdateCenterInput{}
		if in.Body.Plan != "" {
			upd.Plan = &in.Body.Plan
		}
		if in.Body.Status != "" {
			upd.Status = &in.Body.Status
		}
		cs, err := super.UpdateCenter(ctx, id, upd)
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		return &centerSummaryOutput{Body: toCenterSummary(cs)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "admin-centers-billing",
		Method:      http.MethodPatch,
		Path:        "/admin/centers/{id}/billing",
		Summary:     "Set a center's billing (plan tier, trial/active/expired, trial end). Payme/Click is post-MVP.",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *setBillingInput) (*centerDetailOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid center id")
		}
		upd := superadminusecase.SetBillingInput{}
		if in.Body.Plan != "" {
			upd.Plan = &in.Body.Plan
		}
		if in.Body.BillingStatus != "" {
			upd.BillingStatus = &in.Body.BillingStatus
		}
		if in.Body.TrialEndsAt != "" {
			t, terr := parseOptDate(in.Body.TrialEndsAt)
			if terr != nil {
				return nil, huma.Error422UnprocessableEntity("invalid trialEndsAt")
			}
			upd.TrialEndsAt = t
		}
		cs, err := super.SetBilling(ctx, id, upd)
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		return &centerDetailOutput{Body: toCenterDetail(cs)}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID:   "admin-centers-delete",
		Method:        http.MethodDelete,
		Path:          "/admin/centers/{id}",
		Summary:       "Delete a center (cascades all its data)",
		Tags:          []string{"admin"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnprocessableEntity, http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, in *centerIDInput) (*noContentOutput, error) {
		id, perr := uuid.Parse(in.ID)
		if perr != nil {
			return nil, huma.Error422UnprocessableEntity("invalid center id")
		}
		if err := super.DeleteCenter(ctx, id); err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		return &noContentOutput{}, nil
	})

	Register(api, BearerOperation(huma.Operation{
		OperationID: "admin-metrics",
		Method:      http.MethodGet,
		Path:        "/admin/metrics",
		Summary:     "Global platform metrics",
		Tags:        []string{"admin"},
		Errors:      []int{http.StatusForbidden, http.StatusInternalServerError},
	}), func(ctx context.Context, _ *meEmptyInput) (*metricsOutput, error) {
		m, err := super.Metrics(ctx)
		if err != nil {
			return nil, mapAdminError(LangFromContext(ctx), err, log)
		}
		resp := metricsResponse{
			TotalCenters:     m.TotalCenters,
			ActiveCenters:    m.ActiveCenters,
			SuspendedCenters: m.SuspendedCenters,
			TotalStudents:    m.TotalStudents,
			TotalAtRisk:      m.TotalAtRisk,
			CentersByPlan:    make([]planCount, 0, len(m.CentersByPlan)),
			StudentsByTier:   tierCounts{Green: m.Green, Yellow: m.Yellow, Red: m.Red},
			RecentCenters:    make([]centerSummary, 0, len(m.Recent)),
		}
		for plan, count := range m.CentersByPlan {
			resp.CentersByPlan = append(resp.CentersByPlan, planCount{Plan: plan, Count: count})
		}
		for _, c := range m.Recent {
			resp.RecentCenters = append(resp.RecentCenters, toCenterSummary(c))
		}
		return &metricsOutput{Body: resp}, nil
	})
}

func toCenterSummary(cs repo.CenterStats) centerSummary {
	trialEnds := ""
	if cs.Org.TrialEndsAt != nil {
		trialEnds = cs.Org.TrialEndsAt.UTC().Format(time.RFC3339)
	}
	return centerSummary{
		ID:            cs.Org.ID.String(),
		Name:          cs.Org.Name,
		Slug:          cs.Org.Slug,
		Plan:          cs.Org.Plan,
		Status:        cs.Org.Status,
		BillingStatus: cs.Org.BillingStatus,
		TrialEndsAt:   trialEnds,
		TrialDaysLeft: cs.Org.TrialDaysLeft(time.Now()),
		StudentCount:  cs.Students,
		UserCount:     cs.Users,
		AtRiskCount:   cs.Yellow + cs.Red,
		CreatedAt:     cs.Org.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toCenterDetail(cs repo.CenterStats) centerDetail {
	s := toCenterSummary(cs)
	return centerDetail{
		ID:            s.ID,
		Name:          s.Name,
		Slug:          s.Slug,
		Plan:          s.Plan,
		Status:        s.Status,
		StudentCount:  s.StudentCount,
		UserCount:     s.UserCount,
		AtRiskCount:   s.AtRiskCount,
		CreatedAt:     s.CreatedAt,
		BillingStatus: s.BillingStatus,
		TrialEndsAt:   s.TrialEndsAt,
		TrialDaysLeft: s.TrialDaysLeft,
		GreenCount:    cs.Green,
		YellowCount:   cs.Yellow,
		RedCount:      cs.Red,
	}
}

func centerMatches(c repo.CenterStats, query, plan, status string) bool {
	if plan != "" && c.Org.Plan != plan {
		return false
	}
	// The default ("all") view hides archived (soft-deleted) centers; they're only shown when
	// explicitly filtered by status=archived.
	if status == "" {
		if c.Org.Status == "archived" {
			return false
		}
	} else if c.Org.Status != status {
		return false
	}
	if q := strings.ToLower(strings.TrimSpace(query)); q != "" {
		if !strings.Contains(strings.ToLower(c.Org.Name), q) && !strings.Contains(strings.ToLower(c.Org.Slug), q) {
			return false
		}
	}
	return true
}

var msgCenterNotFound = i18n.Message{UZ: "Markaz topilmadi.", RU: "Центр не найден.", EN: "Center not found."}

func mapAdminError(lang i18n.Lang, err error, log zerolog.Logger) error {
	switch {
	case errors.Is(err, superadminusecase.ErrValidation):
		return huma.Error422UnprocessableEntity(err.Error())
	case errors.Is(err, superadminusecase.ErrSlugTaken):
		return huma.Error409Conflict(msgSlugTaken.For(lang))
	case errors.Is(err, superadminusecase.ErrNotFound), errors.Is(err, repo.ErrNotFound):
		return huma.Error404NotFound(msgCenterNotFound.For(lang))
	default:
		log.Error().Err(err).Msg("admin: unexpected error")
		return huma.Error500InternalServerError(msgInternal.For(lang))
	}
}
