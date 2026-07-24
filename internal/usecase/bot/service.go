// Package bot is the Telegram notification bot. ONE platform bot serves every center (multi-tenant)
// as a ONE-WAY push channel: homework deadline reminders, weekly check-in nudges, and motivational
// messages. All student interaction happens in the Mini App — the bot never runs a chat flow. A
// student's chat is bound to their record from the Mini App's signed initData (LinkTelegram); pushes
// then go to that chat id.
package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/telegram"
	"github.com/student-success/backend/internal/repo"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

// ErrNotConfigured signals the bot has no token configured.
var ErrNotConfigured = errors.New("bot: telegram not configured")

// StudentService is the slice of the student usecase the bot needs.
type StudentService interface {
	Get(ctx context.Context, orgID, id uuid.UUID) (*studentusecase.Detail, error)
	SubmitWeeklySurvey(ctx context.Context, orgID, studentID uuid.UUID, motivation, progress int, obstacle, comment string) (entity.Survey, error)
}

// Sender is the slice of the Telegram client the bot uses (interface so pushes are testable).
type Sender interface {
	SendMessage(ctx context.Context, chatID int64, text string, markup *telegram.InlineKeyboardMarkup) error
	AnswerCallback(ctx context.Context, callbackID, text string) error
	Configured() bool
}

type Service struct {
	tg       Sender
	bots     repo.BotRepository
	students StudentService
	username string // bot @username
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

// BroadcastWeeklySurvey pushes the weekly check-in reminder (+ Mini App button) to every linked
// student. Returns the number of chats messaged; a no-op (0) when the bot isn't configured.
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

// RemindDueHomework pushes a "deadline soon" nudge to each linked student in one org who hasn't
// submitted an assignment due within the next 2 hours, then marks those assignments reminded so
// they aren't pinged again. Returns the number of pushes sent.
func (s *Service) RemindDueHomework(ctx context.Context, orgID uuid.UUID) (int, error) {
	if !s.tg.Configured() {
		return 0, nil
	}
	reminders, err := s.bots.DueHomeworkReminders(ctx, orgID)
	if err != nil {
		return 0, err
	}
	marked := map[uuid.UUID]bool{}
	for _, r := range reminders {
		s.send(ctx, r.ChatID, fmt.Sprintf(msgHomeworkDeadline, r.Title), openAppKeyboard())
		if !marked[r.AssignmentID] {
			marked[r.AssignmentID] = true
			if err := s.bots.MarkHomeworkReminded(ctx, orgID, r.AssignmentID); err != nil {
				s.log.Warn().Err(err).Msg("bot: mark homework reminded")
			}
		}
	}
	return len(reminders), nil
}

// HandleUpdate dispatches one Telegram update. It never returns an error — failures are logged — so
// one bad update can't stop the bot.
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

// handleCallback acks any stale inline-button tap left over from older interactive messages and
// points the student to the Mini App. The push-only bot sends no actionable buttons of its own.
func (s *Service) handleCallback(ctx context.Context, cb *telegram.CallbackQuery) {
	if cb.Message == nil {
		return
	}
	_ = s.tg.AnswerCallback(ctx, cb.ID, "")
	s.send(ctx, cb.Message.Chat.ID, msgUseApp, openAppKeyboard())
}

// openAppKeyboard is a single inline button that launches the student Mini App.
func openAppKeyboard() *telegram.InlineKeyboardMarkup {
	return &telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{{
			{Text: "📱 Ilovani ochish", WebApp: &telegram.WebAppInfo{URL: miniAppURL}},
		}},
	}
}

func (s *Service) send(ctx context.Context, chatID int64, text string, kb *telegram.InlineKeyboardMarkup) {
	if err := s.tg.SendMessage(ctx, chatID, text, kb); err != nil {
		s.log.Error().Err(err).Int64("chat", chatID).Msg("bot: send message")
	}
}
