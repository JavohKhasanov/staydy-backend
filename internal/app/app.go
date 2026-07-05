// Package app is the composition root: it builds every dependency in order and runs the
// HTTP server plus background workers under a single errgroup.
package app

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/student-success/backend/internal/config"
	httpctrl "github.com/student-success/backend/internal/controller/http"
	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/observability"
	"github.com/student-success/backend/internal/platform/gemini"
	"github.com/student-success/backend/internal/platform/postgres"
	"github.com/student-success/backend/internal/platform/telegram"
	"github.com/student-success/backend/internal/repo"
	repopg "github.com/student-success/backend/internal/repo/postgres"
	"github.com/student-success/backend/internal/security"
	adviceusecase "github.com/student-success/backend/internal/usecase/advice"
	authusecase "github.com/student-success/backend/internal/usecase/auth"
	botusecase "github.com/student-success/backend/internal/usecase/bot"
	dashboardusecase "github.com/student-success/backend/internal/usecase/dashboard"
	activityusecase "github.com/student-success/backend/internal/usecase/activity"
	courseusecase "github.com/student-success/backend/internal/usecase/course"
	enrollmentusecase "github.com/student-success/backend/internal/usecase/enrollment"
	financeusecase "github.com/student-success/backend/internal/usecase/finance"
	groupusecase "github.com/student-success/backend/internal/usecase/group"
	leadusecase "github.com/student-success/backend/internal/usecase/lead"
	lessonusecase "github.com/student-success/backend/internal/usecase/lesson"
	interventionusecase "github.com/student-success/backend/internal/usecase/intervention"
	branchusecase "github.com/student-success/backend/internal/usecase/branch"
	obstacleusecase "github.com/student-success/backend/internal/usecase/obstacle"
	roomusecase "github.com/student-success/backend/internal/usecase/room"
	maintenanceusecase "github.com/student-success/backend/internal/usecase/maintenance"
	salaryusecase "github.com/student-success/backend/internal/usecase/salary"
	signupusecase "github.com/student-success/backend/internal/usecase/signup"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
	superadminusecase "github.com/student-success/backend/internal/usecase/superadmin"
	teacherusecase "github.com/student-success/backend/internal/usecase/teacher"
)

type Application struct {
	log         zerolog.Logger
	db          *postgres.DB
	server      *httpctrl.Server
	sessions    repo.SessionRepository
	telegram    *telegram.Client
	bot         *botusecase.Service
	maintenance *maintenanceusecase.Service
}

// New boots dependencies: logger → db → metrics → security → repos → usecases → server.
func New(ctx context.Context, cfg *config.Config) (*Application, error) {
	log := observability.NewLogger(cfg.Observability.LogLevel)

	db, err := postgres.New(ctx, cfg.Database.URL)
	if err != nil {
		return nil, err
	}

	metrics, err := observability.NewMetrics()
	if err != nil {
		db.Close()
		return nil, err
	}

	tokenManager := security.NewTokenManager(cfg.Auth.Issuer, cfg.Auth.SigningKey, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL)

	authRepo := repopg.NewAuthRepository(db)
	sessionRepo := repopg.NewSessionRepository(db)
	studentRepo := repopg.NewStudentRepository(db)
	surveyRepo := repopg.NewSurveyRepository(db)
	attendanceRepo := repopg.NewAttendanceRepository(db)
	homeworkRepo := repopg.NewHomeworkRepository(db)
	interventionRepo := repopg.NewInterventionRepository(db)
	retentionRepo := repopg.NewRetentionRepository(db)
	dashboardRepo := repopg.NewDashboardRepository(db)
	groupRepo := repopg.NewGroupRepository(db)
	teacherRepo := repopg.NewTeacherRepository(db)

	geminiClient := gemini.New(cfg.Gemini.APIKey, cfg.Gemini.Model, "")
	telegramClient := telegram.New(cfg.Telegram.BotToken, "")
	botRepo := repopg.NewBotRepository(db)

	// Resolve the bot @username once so deep links (t.me/<username>?start=...) are well-formed.
	botUsername := ""
	if telegramClient.Configured() {
		if info, err := telegramClient.GetMe(ctx); err != nil {
			log.Warn().Err(err).Msg("telegram getMe failed; bot links/polling disabled")
		} else {
			botUsername = info.Username
			log.Info().Str("bot", "@"+botUsername).Msg("telegram bot ready")
		}
	}

	authService := authusecase.NewService(authRepo, sessionRepo, tokenManager)
	studentService := studentusecase.NewService(studentRepo, surveyRepo, attendanceRepo, homeworkRepo, retentionRepo)
	interventionService := interventionusecase.NewService(interventionRepo)
	groupService := groupusecase.NewService(groupRepo, teacherRepo, studentRepo)
	teacherService := teacherusecase.NewService(teacherRepo)
	obstacleService := obstacleusecase.NewService(repopg.NewObstacleRepository(db))
	courseService := courseusecase.NewService(repopg.NewCourseRepository(db))
	enrollmentService := enrollmentusecase.NewService(repopg.NewEnrollmentRepository(db))
	financeService := financeusecase.NewService(repopg.NewFinanceRepository(db))
	leadService := leadusecase.NewService(repopg.NewLeadRepository(db))
	activityService := activityusecase.NewService(repopg.NewActivityRepository(db))
	lessonService := lessonusecase.NewService(repopg.NewLessonRepository(db))
	dashboardService := dashboardusecase.NewService(dashboardRepo)
	adviceService := adviceusecase.NewService(geminiClient)
	botService := botusecase.NewService(telegramClient, botRepo, studentService, botUsername, log)
	maintenanceService := maintenanceusecase.NewService(authRepo, studentService, botService, log)
	superadminService := superadminusecase.NewService(repopg.NewSuperadminRepository(db), authRepo)
	signupService := signupusecase.NewService(repopg.NewSignupRepository(db))
	salaryService := salaryusecase.NewService(repopg.NewSalaryRepository(db))
	branchService := branchusecase.NewService(repopg.NewBranchRepository(db))
	roomService := roomusecase.NewService(repopg.NewRoomRepository(db))

	// Seed the platform owner account (idempotent; no-op unless SUPERADMIN_* are set).
	bootstrapSuperadmin(ctx, authRepo, cfg.Superadmin, log)

	server := httpctrl.NewServer(httpctrl.Dependencies{
		Config:        cfg,
		Logger:        log,
		Metrics:       metrics,
		TokenManager:  tokenManager,
		Auth:          authService,
		Students:      studentService,
		Interventions: interventionService,
		Dashboard:     dashboardService,
		Advice:        adviceService,
		Bot:           botService,
		Groups:        groupService,
		Teachers:      teacherService,
		Obstacles:     obstacleService,
		Courses:       courseService,
		Enrollments:   enrollmentService,
		Finance:       financeService,
		Leads:         leadService,
		Activities:    activityService,
		Lessons:       lessonService,
		Superadmin:    superadminService,
		Signup:        signupService,
		Salary:        salaryService,
		Branches:      branchService,
		Rooms:         roomService,
	})

	return &Application{log: log, db: db, server: server, sessions: sessionRepo, telegram: telegramClient, bot: botService, maintenance: maintenanceService}, nil
}

// Run starts the HTTP server + background workers and blocks until ctx is canceled.
func (a *Application) Run(ctx context.Context) error {
	defer a.db.Close()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return a.server.Run(ctx) })
	g.Go(func() error { return a.runSessionReaper(ctx) })
	g.Go(func() error { return a.runBotPoller(ctx) })
	g.Go(func() error { return a.runScheduler(ctx) })
	return g.Wait()
}

// runBotPoller long-polls Telegram for updates and dispatches them to the bot usecase. It's a
// no-op when no bot token is configured. Long polling (vs webhook) needs no public URL, so it
// works in local dev; webhook mode can be added later for production scale.
func (a *Application) runBotPoller(ctx context.Context) error {
	if a.telegram == nil || !a.telegram.Configured() {
		a.log.Info().Msg("telegram bot poller disabled (no token)")
		return nil
	}
	// Long polling and webhooks are mutually exclusive — drop any stale webhook first.
	if err := a.telegram.DeleteWebhook(ctx); err != nil {
		a.log.Warn().Err(err).Msg("telegram deleteWebhook")
	}
	a.log.Info().Msg("telegram bot poller started")

	var offset int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		updates, err := a.telegram.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			a.log.Warn().Err(err).Msg("telegram getUpdates")
			if !sleepCtx(ctx, 3*time.Second) {
				return nil
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			a.handleUpdateSafely(ctx, u.UpdateID, func() { a.bot.HandleUpdate(ctx, u) })
		}
	}
}

// handleUpdateSafely runs one update handler, recovering from panics so a single malformed
// update can never take down the poller.
func (a *Application) handleUpdateSafely(_ context.Context, updateID int64, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			a.log.Error().Interface("panic", r).Int64("update", updateID).Msg("telegram update handler panicked")
		}
	}()
	fn()
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// schedulerHour is the local hour-of-day the daily job runs (Monday also pushes the survey).
const schedulerHour = 9

// runScheduler runs the daily maintenance job at schedulerHour; on Mondays it also broadcasts
// the weekly check-in survey. It recomputes every tenant's risk so time-based trigger rules fire
// without a triggering write.
func (a *Application) runScheduler(ctx context.Context) error {
	a.log.Info().Int("hour", schedulerHour).Msg("scheduler started (daily recompute; Monday survey push)")
	for {
		next := nextDailyRun(time.Now(), schedulerHour)
		if !sleepCtx(ctx, time.Until(next)) {
			return nil
		}
		if next.Weekday() == time.Monday {
			a.maintenance.RunWeekly(context.Background())
		} else {
			a.maintenance.RunDaily(context.Background())
		}
	}
}

// nextDailyRun returns the next occurrence of hour:00 local time strictly after now.
func nextDailyRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// bootstrapSuperadmin ensures the reserved platform org + super_admin account exists. Idempotent
// (no-op once the platform org exists, or when SUPERADMIN_EMAIL/PASSWORD aren't set).
func bootstrapSuperadmin(ctx context.Context, authRepo repo.AuthRepository, cfg config.Superadmin, log zerolog.Logger) {
	if cfg.Email == "" || cfg.Password == "" {
		return
	}
	exists, err := authRepo.SlugExists(ctx, superadminusecase.PlatformSlug)
	if err != nil {
		log.Error().Err(err).Msg("superadmin bootstrap: slug check")
		return
	}
	if exists {
		return
	}
	hash, err := security.HashPassword(cfg.Password)
	if err != nil {
		log.Error().Err(err).Msg("superadmin bootstrap: hash")
		return
	}
	if _, _, err := authRepo.ProvisionOrgWithAdmin(ctx, repo.ProvisionParams{
		OrgName:      "Platform",
		Slug:         superadminusecase.PlatformSlug,
		Plan:         "internal",
		Email:        cfg.Email,
		PasswordHash: hash,
		FullName:     cfg.FullName,
		Role:         entity.RoleSuperAdmin,
	}); err != nil {
		log.Error().Err(err).Msg("superadmin bootstrap: provision")
		return
	}
	log.Info().Str("email", cfg.Email).Msg("superadmin account bootstrapped")
}

// runSessionReaper periodically prunes expired refresh sessions so the table doesn't grow
// unbounded.
func (a *Application) runSessionReaper(ctx context.Context) error {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := a.sessions.DeleteExpired(context.Background()); err != nil {
				a.log.Warn().Err(err).Msg("session reaper: delete expired")
			}
		}
	}
}
