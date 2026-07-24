// Package bot is the Telegram retention bot usecase. ONE platform bot serves every center
// (multi-tenant): a student links their chat via a per-student deep link, then runs a weekly
// check-in (motivation / progress / obstacle / comment) that feeds the same retention pipeline
// as the web API. Incoming updates carry no tenant scope, so the chat→org/student binding is
// resolved from the non-RLS bot tables first, and only the survey write enters WithTenant.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/telegram"
	"github.com/student-success/backend/internal/repo"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

// ErrNotConfigured signals the bot has no token / username (so links can't be issued).
var ErrNotConfigured = errors.New("bot: telegram not configured")

// inviteTTL is how long a deep-link invite stays valid.
const inviteTTL = 7 * 24 * time.Hour

// StudentService is the slice of the student usecase the bot needs.
type StudentService interface {
	Get(ctx context.Context, orgID, id uuid.UUID) (*studentusecase.Detail, error)
	SubmitWeeklySurvey(ctx context.Context, orgID, studentID uuid.UUID, motivation, progress int, obstacle, comment string) (entity.Survey, error)
}

// Sender is the slice of the Telegram client the bot uses (interface so the FSM is testable).
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup *telegram.InlineKeyboardMarkup) error
	AnswerCallback(ctx context.Context, callbackID, text string) error
	Configured() bool
}

type Service struct {
	tg       Sender
	bots     repo.BotRepository
	students StudentService
	username string // bot @username, for building deep links
	token    string // bot token, for verifying Mini App initData signatures
	log      zerolog.Logger
}

func NewService(tg Sender, bots repo.BotRepository, students StudentService, username, token string, log zerolog.Logger) *Service {
	return &Service{tg: tg, bots: bots, students: students, username: username, token: token, log: log}
}

// LinkTelegram binds a signed-in student's Telegram to their record from Mini App initData, so the
// bot may later push notifications to them. The initData signature is verified against the bot token
// (proves the caller really is that Telegram user); the id doubles as the private-chat id.
func (s *Service) LinkTelegram(ctx context.Context, orgID, studentID uuid.UUID, initData string) error {
	tgID, err := telegram.VerifyInitData(initData, s.token)
	if err != nil {
		return err
	}
	if err := s.bots.SetStudentChat(ctx, orgID, studentID, tgID); err != nil {
		return err
	}
	// Keep bot_users in sync so an inbound message resolves to the student too (best-effort).
	if err := s.bots.BindChat(ctx, tgID, orgID, studentID); err != nil {
		s.log.Warn().Err(err).Msg("bot: bind chat on telegram link")
	}
	return nil
}

// InviteLink is a deep link the admin shares with a student to connect their Telegram.
type InviteLink struct {
	Link      string
	Token     string
	ExpiresAt time.Time
}

// CreateInviteLink verifies the student belongs to the caller's tenant (Get is RLS-scoped →
// ErrNotFound otherwise), mints a one-time token, and returns a t.me deep link.
func (s *Service) CreateInviteLink(ctx context.Context, orgID, studentID, createdBy uuid.UUID) (InviteLink, error) {
	if !s.tg.Configured() || s.username == "" {
		return InviteLink{}, ErrNotConfigured
	}
	if _, err := s.students.Get(ctx, orgID, studentID); err != nil {
		return InviteLink{}, err // studentusecase.ErrNotFound propagates → 404
	}
	token, err := randomToken()
	if err != nil {
		return InviteLink{}, err
	}
	expiresAt := time.Now().Add(inviteTTL)
	if err := s.bots.CreateInvite(ctx, token, orgID, studentID, createdBy, expiresAt); err != nil {
		return InviteLink{}, err
	}
	return InviteLink{
		Link:      fmt.Sprintf("https://t.me/%s?start=%s", s.username, token),
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// BroadcastWeeklySurvey starts a check-in for every linked student (the Monday survey push).
// Returns the number of chats messaged; a no-op (0) when the bot isn't configured. Per-chat
// send failures are logged inside startCheckin and don't abort the broadcast.
func (s *Service) BroadcastWeeklySurvey(ctx context.Context) (int, error) {
	if !s.tg.Configured() {
		return 0, nil
	}
	chats, err := s.bots.ListLinkedChats(ctx)
	if err != nil {
		return 0, err
	}
	for _, c := range chats {
		s.send(ctx, c.ChatID, msgWeeklyReminder, openAppKeyboard())
	}
	return len(chats), nil
}

func randomToken() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

const (
	flowCheckin  = "checkin"
	stepMotiv    = "motivation"
	stepProgress = "progress"
	stepObstacle = "obstacle"
	stepComment  = "comment"
)

// obstacles is the default choice set, used when a center hasn't configured its own (or a load
// fails). Centers override it via obstacle_options; the keyboard's callback index references the
// SAME list it was built from, so options must be stable for the duration of one check-in.
var obstacles = []string{
	"Vaqt yetishmasligi", "Ish", "Oila", "Darslarni tushunmaslik",
	"Moliyaviy muammo", "Motivatsiya pastligi", "Boshqa",
}

type draft struct {
	Motivation int    `json:"m"`
	Progress   int    `json:"p"`
	Obstacle   string `json:"o"`
}

// HandleUpdate dispatches one Telegram update. It never returns an error — failures are logged and,
// where possible, surfaced to the user as a friendly message — so one bad update can't stop the bot.
func (s *Service) HandleUpdate(ctx context.Context, u telegram.Update) {
	switch {
	case u.Message != nil && u.Message.Chat.ID != 0:
		s.handleMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		s.handleCallback(ctx, u.CallbackQuery)
	}
}

// handleMessage: the bot is a one-way notification channel. Everything the student does happens in
// the Mini App, so /start greets + opens the app, and any other message just points back to it
// (never silently ignored — a reply with the app button is clearer than silence).
func (s *Service) handleMessage(ctx context.Context, m *telegram.Message) {
	chatID := m.Chat.ID
	if strings.HasPrefix(strings.TrimSpace(m.Text), "/start") {
		s.send(ctx, chatID, msgStart, openAppKeyboard())
		return
	}
	s.send(ctx, chatID, msgUseApp, openAppKeyboard())
}

// openAppKeyboard is a single inline button that launches the student Mini App.
func openAppKeyboard() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{{
			{Text: "📱 Ilovani ochish", WebApp: &telegram.WebAppInfo{URL: miniAppURL}},
		}},
	}
}

// linkChat binds the chat to the student named in a deep-link token, then offers a check-in.
func (s *Service) linkChat(ctx context.Context, chatID int64, token string) {
	invite, err := s.bots.ResolveInvite(ctx, token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.send(ctx, chatID, msgBadToken, nil)
			return
		}
		s.fail(ctx, chatID, "resolve invite", err)
		return
	}
	if invite.Used {
		s.send(ctx, chatID, msgUsedToken, nil)
		return
	}
	if invite.Expired {
		s.send(ctx, chatID, msgExpiredToken, nil)
		return
	}

	if err := s.bots.BindChat(ctx, chatID, invite.OrgID, invite.StudentID); err != nil {
		s.fail(ctx, chatID, "bind chat", err)
		return
	}
	if err := s.bots.SetStudentChat(ctx, invite.OrgID, invite.StudentID, chatID); err != nil {
		s.fail(ctx, chatID, "set student chat", err)
		return
	}
	if err := s.bots.UseInvite(ctx, token); err != nil {
		// Non-fatal: the chat is already bound; just log.
		s.log.Warn().Err(err).Msg("bot: mark invite used")
	}

	name := "talaba"
	if d, err := s.students.Get(ctx, invite.OrgID, invite.StudentID); err == nil && d.Student.Name != "" {
		name = d.Student.Name
	}
	s.send(ctx, chatID, fmt.Sprintf(msgWelcome, name), nil)
	s.startCheckin(ctx, chatID)
}

func (s *Service) greet(ctx context.Context, chatID int64) {
	if _, err := s.bots.GetConversation(ctx, chatID); err != nil {
		s.send(ctx, chatID, msgNotLinked, nil)
		return
	}
	s.send(ctx, chatID, msgAlreadyLinked, nil)
}

func (s *Service) startCheckin(ctx context.Context, chatID int64) {
	conv, err := s.bots.GetConversation(ctx, chatID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.send(ctx, chatID, msgNotLinked, nil)
			return
		}
		s.fail(ctx, chatID, "get conversation", err)
		return
	}
	if conv.StudentID == uuid.Nil {
		s.send(ctx, chatID, msgNotLinked, nil)
		return
	}
	if err := s.bots.SetFlow(ctx, chatID, flowCheckin, stepMotiv, mustJSON(draft{})); err != nil {
		s.fail(ctx, chatID, "set flow", err)
		return
	}
	s.send(ctx, chatID, msgAskMotivation, scoreKeyboard("m"))
}

// showProfile sends the linked student their own status: risk zone (by colour, no number — TZ
// §4.1.3), attendance summary, and survey count.
func (s *Service) showProfile(ctx context.Context, chatID int64) {
	conv, err := s.bots.GetConversation(ctx, chatID)
	if err != nil || conv.StudentID == uuid.Nil {
		s.send(ctx, chatID, msgNotLinked, nil)
		return
	}
	detail, err := s.students.Get(ctx, conv.OrgID, conv.StudentID)
	if err != nil {
		s.fail(ctx, chatID, "get profile", err)
		return
	}
	s.send(ctx, chatID, buildProfile(detail), nil)
}

// buildProfile renders the student-facing profile. It deliberately shows the risk ZONE (colour),
// never the numeric score.
func buildProfile(d *studentusecase.Detail) string {
	var b strings.Builder
	fmt.Fprintf(&b, "👤 *%s*\n\n%s", d.Student.Name, zoneText(d.Student.RiskTier))
	if total := len(d.Attendance); total > 0 {
		present := 0
		for _, a := range d.Attendance {
			if a.IsPresent {
				present++
			}
		}
		fmt.Fprintf(&b, "\n\n📅 Davomat: %d darsdan %d tasiga kelgansiz.", total, present)
	} else {
		b.WriteString("\n\n📅 Davomat: hali ma'lumot yo'q.")
	}
	fmt.Fprintf(&b, "\n📝 To'ldirilgan so'rovnomalar: %d ta.", len(d.Surveys))
	b.WriteString("\n\nHaftalik holatni belgilash uchun /checkin yuboring.")
	return b.String()
}

func zoneText(tier string) string {
	switch tier {
	case "Red":
		return "🔴 *Qizil zona* — hozir biroz qiyin. Mentoringiz tez orada siz bilan bog'lanadi, xavotir olmang."
	case "Yellow":
		return "🟡 *Sariq zona* — biroz e'tibor bering. Kichik qadamlar bilan yana yo'lga tushasiz. 🙂"
	default:
		return "🟢 *Yashil zona* — holatingiz yaxshi! Shu zayl davom eting. 💪"
	}
}

// handleCallback acks any stale inline-button tap left over from older interactive messages and
// points the student to the Mini App. The push-only bot sends no actionable buttons of its own.
func (s *Service) handleCallback(ctx context.Context, cb *telegram.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	_ = s.tg.AnswerCallback(ctx, cb.ID, "")
	s.send(ctx, cb.Message.Chat.ID, msgUseApp, openAppKeyboard())
}

func (s *Service) advance(ctx context.Context, chatID int64, nextStep string, d draft, prompt string, kb *telegram.InlineKeyboardMarkup) {
	if err := s.bots.SetFlow(ctx, chatID, flowCheckin, nextStep, mustJSON(d)); err != nil {
		s.fail(ctx, chatID, "set flow", err)
		return
	}
	s.send(ctx, chatID, prompt, kb)
}

// handleFlowText handles free-text steps (the comment) and stray messages.
func (s *Service) handleFlowText(ctx context.Context, chatID int64, text string) {
	conv, err := s.bots.GetConversation(ctx, chatID)
	if err != nil {
		s.send(ctx, chatID, msgNotLinked, nil)
		return
	}
	if conv.Flow != flowCheckin || conv.Step != stepComment {
		s.send(ctx, chatID, msgUseCheckin, nil)
		return
	}

	comment := text
	if text == "/skip" {
		comment = ""
	}
	var d draft
	_ = json.Unmarshal(conv.State, &d)

	if _, err := s.students.SubmitWeeklySurvey(ctx, conv.OrgID, conv.StudentID, d.Motivation, d.Progress, d.Obstacle, comment); err != nil {
		s.fail(ctx, chatID, "submit survey", err)
		return
	}
	if err := s.bots.SetFlow(ctx, chatID, "", "", nil); err != nil {
		s.log.Warn().Err(err).Msg("bot: clear flow")
	}
	s.send(ctx, chatID, msgDone, nil)
}

// --- helpers ---

func (s *Service) send(ctx context.Context, chatID int64, text string, kb *telegram.InlineKeyboardMarkup) {
	if err := s.tg.SendMessage(ctx, chatID, text, kb); err != nil {
		s.log.Error().Err(err).Int64("chat", chatID).Msg("bot: send message")
	}
}

func (s *Service) fail(ctx context.Context, chatID int64, op string, err error) {
	s.log.Error().Err(err).Str("op", op).Int64("chat", chatID).Msg("bot: handler error")
	s.send(ctx, chatID, msgError, nil)
}

func scoreKeyboard(prefix string) *telegram.InlineKeyboardMarkup {
	row := make([]telegram.InlineKeyboardButton, 0, 5)
	for i := 1; i <= 5; i++ {
		n := strconv.Itoa(i)
		row = append(row, telegram.InlineKeyboardButton{Text: n, CallbackData: prefix + ":" + n})
	}
	return &telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{row}}
}

// obstaclesFor returns the center's configured obstacle choices, falling back to the built-in
// defaults when none are set (or on a read error — the check-in must never get stuck).
func (s *Service) obstaclesFor(ctx context.Context, orgID uuid.UUID) []string {
	opts, err := s.bots.ListObstacleLabels(ctx, orgID)
	if err != nil {
		s.log.Warn().Err(err).Msg("bot: load obstacle options; using defaults")
		return obstacles
	}
	if len(opts) == 0 {
		return obstacles
	}
	return opts
}

func obstacleKeyboard(labels []string) *telegram.InlineKeyboardMarkup {
	rows := make([][]telegram.InlineKeyboardButton, 0, len(labels))
	for i, o := range labels {
		rows = append(rows, []telegram.InlineKeyboardButton{
			{Text: o, CallbackData: "o:" + strconv.Itoa(i)},
		})
	}
	return &telegram.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func atoiClamp(s string, lo, hi int) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func mustJSON(d draft) []byte {
	b, _ := json.Marshal(d)
	return b
}
