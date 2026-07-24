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
	homeworkusecase "github.com/student-success/backend/internal/usecase/homework"
	pointsusecase "github.com/student-success/backend/internal/usecase/points"
	salaryusecase "github.com/student-success/backend/internal/usecase/salary"
	shopusecase "github.com/student-success/backend/internal/usecase/shop"
	staffusecase "github.com/student-success/backend/internal/usecase/staff"
	studentauthusecase "github.com/student-success/backend/internal/usecase/studentauth"
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
	StudentAuth   *studentauthusecase.Service
	Homework      *homeworkusecase.Service
	Shop          *shopusecase.Service
	Points        *pointsusecase.Service
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
	// Back-office only: a student token must never reach tenant back-office endpoints (some groups
	// gate on RequireAuth alone), so exclude the student role here.
	protectedAPI.UseMiddleware(RequireRole(api, entity.BackOfficeRoles()...))

	registerAuth(publicAPI, deps.Auth, deps.Logger)
	registerAdminAuth(publicAPI, deps.Auth, deps.Logger)   // superadmin login (public)
	registerStudentAuth(publicAPI, deps.StudentAuth, deps.Logger) // student mini-app login (public)

	// Student mini app: the signed-in student's own data, gated to the student role.
	studentAPI := huma.NewGroup(api, "/api/v1")
	studentAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	studentAPI.UseMiddleware(RequireRole(api, entity.RoleStudent))
	registerStudentApp(studentAPI, deps.Students, deps.Points, deps.Logger)
	registerStudentHomework(studentAPI, deps.Homework, deps.Logger)
	registerStudentShop(studentAPI, deps.Shop, deps.Points, deps.Logger)
	registerStudentData(studentAPI, deps.Groups, deps.Finance, deps.Logger)

	// Homework management + the staff leaderboard: teachers (scoped to their own groups) plus
	// admins/managers.
	teachingAPI := huma.NewGroup(api, "/api/v1")
	teachingAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	teachingAPI.UseMiddleware(RequireRole(api, entity.RoleTeacher, entity.RoleManager, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerHomework(teachingAPI, deps.Homework, deps.Logger)
	registerLeaderboard(teachingAPI, deps.Points, deps.Logger)
	registerManualXP(teachingAPI, deps.Points, deps.Groups, deps.Logger)
	registerStudentCredentials(teachingAPI, deps.Students, deps.Groups, deps.Logger)
	registerStudents(protectedAPI, deps.Students, deps.Logger)
	registerInterventions(protectedAPI, deps.Interventions, deps.Logger)
	registerDashboard(protectedAPI, deps.Dashboard, deps.Logger)
	registerAdvice(protectedAPI, deps.Students, deps.Advice, deps.Logger)
	registerTelegram(protectedAPI, deps.Bot, deps.Logger)
	registerAccount(protectedAPI, deps.Auth, deps.Logger) // self-service change-password (any role)

	// Sensitive money — center profit/expense summary, the expense ledger, and teacher salaries.
	// Director (center_admin) + finance only; the front-desk administrator never sees these.
	financeAPI := huma.NewGroup(api, "/api/v1")
	financeAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	financeAPI.UseMiddleware(RequireRole(api, entity.RoleFinance, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerSalary(financeAPI, deps.Salary, deps.Logger)

	// Operational payments — student balances, invoices, collecting fees, debtors, the group fee
	// roster, payment alerts, billing settings. The front-desk administrator (manager) collects
	// payments too, alongside director + finance. registerFinance keeps the sensitive summary +
	// expense endpoints on financeAPI (passed as its second arg).
	paymentsAPI := huma.NewGroup(api, "/api/v1")
	paymentsAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	paymentsAPI.UseMiddleware(RequireRole(api, entity.RoleManager, entity.RoleFinance, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerFinance(paymentsAPI, financeAPI, deps.Finance, deps.Logger)

	// Read-only teacher/group lists — needed by finance and the administrator for debtor/salary/
	// report context, without granting org-structure mutation rights.
	staffReadAPI := huma.NewGroup(api, "/api/v1")
	staffReadAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	staffReadAPI.UseMiddleware(RequireRole(api, entity.RoleManager, entity.RoleFinance, entity.RoleCenterAdmin, entity.RoleSuperAdmin))

	// Operational center management (groups, courses, schedule, attendance, teachers, rooms,
	// settings). Director + administrator(manager); NOT finance (money-only).
	centerStaffAPI := huma.NewGroup(api, "/api/v1")
	centerStaffAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	centerStaffAPI.UseMiddleware(RequireRole(api, entity.RoleManager, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerGroups(centerStaffAPI, staffReadAPI, deps.Groups, deps.Logger)
	registerTeachers(centerStaffAPI, staffReadAPI, deps.Teachers, deps.Logger)
	registerShop(centerStaffAPI, deps.Shop, deps.Logger)
	registerGamification(centerStaffAPI, deps.Points, deps.Logger)
	registerObstacleOptions(centerStaffAPI, deps.Obstacles, deps.Logger)
	registerCourses(centerStaffAPI, deps.Courses, deps.Logger)
	registerEnrollments(centerStaffAPI, deps.Enrollments, deps.Logger)
	registerBranches(centerStaffAPI, deps.Branches, deps.Logger)
	registerRooms(centerStaffAPI, deps.Rooms, deps.Logger)
	registerLeads(centerStaffAPI, deps.Leads, deps.Students, deps.Logger)
	registerActivities(centerStaffAPI, deps.Activities, deps.Logger)
	registerLessons(centerStaffAPI, deps.Lessons, deps.Logger)
	registerMaintenance(centerStaffAPI, deps.Students, deps.Logger)
	registerImport(centerStaffAPI, deps.Students, deps.Logger)

	// Director-only: staff account management (create/manage administrators, finance, co-directors).
	directorAPI := huma.NewGroup(api, "/api/v1")
	directorAPI.UseMiddleware(RequireAuth(api, deps.TokenManager))
	directorAPI.UseMiddleware(RequireRole(api, entity.RoleCenterAdmin, entity.RoleSuperAdmin))
	registerStaff(directorAPI, deps.Staff, deps.Logger)

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
