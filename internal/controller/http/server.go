package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humaecho"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/config"
	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/observability"
	"github.com/student-success/backend/internal/security"
	activityusecase "github.com/student-success/backend/internal/usecase/activity"
	adviceusecase "github.com/student-success/backend/internal/usecase/advice"
	authusecase "github.com/student-success/backend/internal/usecase/auth"
	botusecase "github.com/student-success/backend/internal/usecase/bot"
	dashboardusecase "github.com/student-success/backend/internal/usecase/dashboard"
	groupusecase "github.com/student-success/backend/internal/usecase/group"
	courseusecase "github.com/student-success/backend/internal/usecase/course"
	enrollmentusecase "github.com/student-success/backend/internal/usecase/enrollment"
	financeusecase "github.com/student-success/backend/internal/usecase/finance"
	interventionusecase "github.com/student-success/backend/internal/usecase/intervention"
	leadusecase "github.com/student-success/backend/internal/usecase/lead"
	lessonusecase "github.com/student-success/backend/internal/usecase/lesson"
	branchusecase "github.com/student-success/backend/internal/usecase/branch"
	obstacleusecase "github.com/student-success/backend/internal/usecase/obstacle"
	planusecase "github.com/student-success/backend/internal/usecase/plan"
	roomusecase "github.com/student-success/backend/internal/usecase/room"
	salaryusecase "github.com/student-success/backend/internal/usecase/salary"
	staffusecase "github.com/student-success/backend/internal/usecase/staff"
	signupusecase "github.com/student-success/backend/internal/usecase/signup"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
	superadminusecase "github.com/student-success/backend/internal/usecase/superadmin"
	teacherusecase "github.com/student-success/backend/internal/usecase/teacher"
)

// Dependencies are everything the HTTP layer needs, wired by internal/app.
type Dependencies struct {
	Config        *config.Config
	Logger        zerolog.Logger
	Metrics       *observability.Metrics
	TokenManager  *security.TokenManager
	Auth          *authusecase.Service
	Students      *studentusecase.Service
	Interventions *interventionusecase.Service
	Dashboard     *dashboardusecase.Service
	Advice        *adviceusecase.Service
	Bot           *botusecase.Service
	Groups        *groupusecase.Service
	Teachers      *teacherusecase.Service
	Staff         *staffusecase.Service
	Obstacles     *obstacleusecase.Service
	Courses       *courseusecase.Service
	Enrollments   *enrollmentusecase.Service
	Finance       *financeusecase.Service
	Leads         *leadusecase.Service
	Activities    *activityusecase.Service
	Lessons       *lessonusecase.Service
	Superadmin    *superadminusecase.Service
	Signup        *signupusecase.Service
	Plans         *planusecase.Service
	Salary        *salaryusecase.Service
	Branches      *branchusecase.Service
	Rooms         *roomusecase.Service
}

type Server struct {
	echo *echo.Echo
	cfg  *config.Config
	log  zerolog.Logger
}

func NewServer(deps Dependencies) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	// The frontend is a separate origin → CORS. Origins come from config (default "*"
	// for dev). Bearer JWT lives in the Authorization header, so credentials stay off.
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: deps.Config.HTTP.CORSOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderAccept, echo.HeaderAuthorization, echo.HeaderContentType, "Accept-Language", "ngrok-skip-browser-warning"},
	}))
	e.Use(requestLogger(deps.Logger))

	if deps.Metrics != nil {
		e.Use(deps.Metrics.EchoMiddleware())
		e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(deps.Metrics.Registry(), promhttp.HandlerOpts{})))
	}

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// Huma: OpenAPI 3.1 at /openapi.json|yaml, Swagger-style docs at /docs.
	humaCfg := huma.DefaultConfig("Student Success API", deps.Config.App.Version)
	humaCfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
	api := humaecho.New(e, humaCfg)

	publicAPI := huma.NewGroup(api, "/api/v1")
	protectedAPI := huma.NewGroup(api, "/api/v1")
	protectedAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))

	registerAuth(publicAPI, deps.Auth, deps.Logger)
	registerAdminAuth(publicAPI, deps.Auth, deps.Logger) // superadmin login (public)
	registerStudents(protectedAPI, deps.Students, deps.Logger)
	registerInterventions(protectedAPI, deps.Interventions, deps.Logger)
	registerDashboard(protectedAPI, deps.Dashboard, deps.Logger)
	registerAdvice(protectedAPI, deps.Students, deps.Advice, deps.Logger)
	registerTelegram(protectedAPI, deps.Bot, deps.Logger)

	// Finance panel: finance + salary + reports data, open to the finance role as well as
	// center_admin / super_admin. Read-only teacher/group lists also mount here so the finance
	// pages can render debtor/salary/report context without full org-management access.
	financeAPI := huma.NewGroup(api, "/api/v1")
	financeAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	financeAPI.UseMiddleware(RequireRole(api, entity.RoleFinance, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerFinance(financeAPI, deps.Finance, deps.Logger)
	registerSalary(financeAPI, deps.Salary, deps.Logger)

	// Org-structure management (groups + teachers) is restricted to center_admin / super_admin.
	// The read-only list endpoints are handed financeAPI so finance can read them too.
	centerAdminAPI := huma.NewGroup(api, "/api/v1")
	centerAdminAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	centerAdminAPI.UseMiddleware(RequireRole(api, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerGroups(centerAdminAPI, financeAPI, deps.Groups, deps.Logger)
	registerTeachers(centerAdminAPI, financeAPI, deps.Teachers, deps.Logger)
	registerStaff(centerAdminAPI, deps.Staff, deps.Logger)
	registerObstacleOptions(centerAdminAPI, deps.Obstacles, deps.Logger)
	registerCourses(centerAdminAPI, deps.Courses, deps.Logger)
	registerEnrollments(centerAdminAPI, deps.Enrollments, deps.Logger)
	registerBranches(centerAdminAPI, deps.Branches, deps.Logger)
	registerRooms(centerAdminAPI, deps.Rooms, deps.Logger)
	registerLeads(centerAdminAPI, deps.Leads, deps.Students, deps.Logger)
	registerActivities(centerAdminAPI, deps.Activities, deps.Logger)
	registerLessons(centerAdminAPI, deps.Lessons, deps.Logger)
	registerMaintenance(centerAdminAPI, deps.Students, deps.Logger)
	registerImport(centerAdminAPI, deps.Students, deps.Logger)

	// The signed-in teacher's own dashboard (their groups/students + scoped marking).
	teacherAPI := huma.NewGroup(api, "/api/v1")
	teacherAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	teacherAPI.UseMiddleware(RequireRole(api, entity.RoleTeacher))
	registerMe(teacherAPI, deps.Groups, deps.Students, deps.Logger)

	// Platform-owner (super_admin) cross-tenant center management.
	adminAPI := huma.NewGroup(api, "/api/v1")
	adminAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	adminAPI.UseMiddleware(RequireRole(api, entity.RoleSuperAdmin))
	registerAdmin(adminAPI, deps.Superadmin, deps.Logger)
	registerSignup(publicAPI, adminAPI, deps.Signup, deps.Logger)
	registerPlans(publicAPI, adminAPI, deps.Plans, deps.Logger)
	// More tenant-scoped feature operations register on protectedAPI as they land.

	return &Server{echo: e, cfg: deps.Config, log: deps.Logger}
}

func (s *Server) Run(ctx context.Context) error {
	addr := s.cfg.HTTP.Host + ":" + s.cfg.HTTP.Port

	// Bound how long a connection can tie up a handler goroutine (slowloris, slow clients, a slow
	// upstream like Gemini). WriteTimeout must exceed the Gemini client's 30s call so a legitimate
	// AI response is never cut off mid-flight.
	s.echo.Server.ReadHeaderTimeout = 5 * time.Second
	s.echo.Server.ReadTimeout = 15 * time.Second
	s.echo.Server.WriteTimeout = 60 * time.Second
	s.echo.Server.IdleTimeout = 60 * time.Second

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.echo.Shutdown(shutdownCtx)
	}()

	s.log.Info().Str("addr", addr).Str("env", s.cfg.App.Env).Msg("http server listening")
	if err := s.echo.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
