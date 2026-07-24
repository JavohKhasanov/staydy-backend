package bot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/platform/telegram"
	"github.com/student-success/backend/internal/repo"
	studentusecase "github.com/student-success/backend/internal/usecase/student"
)

// --- fakes ---

type sentMsg struct {
	text string
	kb   *telegram.InlineKeyboardMarkup
}

type fakeSender struct{ sent []sentMsg }

func (f *fakeSender) SendMessage(_ context.Context, _ int64, text string, kb *telegram.InlineKeyboardMarkup) error {
	f.sent = append(f.sent, sentMsg{text: text, kb: kb})
	return nil
}
func (f *fakeSender) AnswerCallback(context.Context, string, string) error { return nil }
func (f *fakeSender) Configured() bool                                     { return true }
func (f *fakeSender) last() string {
	if len(f.sent) == 0 {
		return ""
	}
	return f.sent[len(f.sent)-1].text
}

type fakeBots struct {
	invites   map[string]repo.BotInvite
	convos    map[int64]repo.BotConversation
	used      map[string]bool
	chatSet   map[uuid.UUID]int64
	linked    []repo.LinkedChat
	reminders []entity.HomeworkReminder
	marked    []uuid.UUID
}

func newFakeBots() *fakeBots {
	return &fakeBots{
		invites: map[string]repo.BotInvite{}, convos: map[int64]repo.BotConversation{},
		used: map[string]bool{}, chatSet: map[uuid.UUID]int64{},
	}
}
func (f *fakeBots) CreateInvite(_ context.Context, token string, orgID, studentID, _ uuid.UUID, _ time.Time) error {
	f.invites[token] = repo.BotInvite{Token: token, OrgID: orgID, StudentID: studentID}
	return nil
}
func (f *fakeBots) ResolveInvite(_ context.Context, token string) (repo.BotInvite, error) {
	inv, ok := f.invites[token]
	if !ok {
		return repo.BotInvite{}, repo.ErrNotFound
	}
	inv.Used = f.used[token]
	return inv, nil
}
func (f *fakeBots) UseInvite(_ context.Context, token string) error { f.used[token] = true; return nil }
func (f *fakeBots) BindChat(_ context.Context, chatID int64, orgID, studentID uuid.UUID) error {
	f.convos[chatID] = repo.BotConversation{ChatID: chatID, OrgID: orgID, StudentID: studentID, State: []byte("{}")}
	return nil
}
func (f *fakeBots) GetConversation(_ context.Context, chatID int64) (repo.BotConversation, error) {
	c, ok := f.convos[chatID]
	if !ok {
		return repo.BotConversation{}, repo.ErrNotFound
	}
	return c, nil
}
func (f *fakeBots) SetFlow(_ context.Context, chatID int64, flow, step string, state []byte) error {
	c := f.convos[chatID]
	c.Flow, c.Step, c.State = flow, step, state
	f.convos[chatID] = c
	return nil
}
func (f *fakeBots) SetStudentChat(_ context.Context, _, studentID uuid.UUID, chatID int64) error {
	f.chatSet[studentID] = chatID
	return nil
}
func (f *fakeBots) ListLinkedChats(context.Context) ([]repo.LinkedChat, error) { return f.linked, nil }
func (f *fakeBots) DueHomeworkReminders(context.Context, uuid.UUID) ([]entity.HomeworkReminder, error) {
	return f.reminders, nil
}
func (f *fakeBots) MarkHomeworkReminded(_ context.Context, _, id uuid.UUID) error {
	f.marked = append(f.marked, id)
	return nil
}
func (f *fakeBots) ListObstacleLabels(context.Context, uuid.UUID) ([]string, error) {
	return nil, nil
}

type fakeStudents struct {
	name      string
	submitted *submittedSurvey
}
type submittedSurvey struct {
	motivation, progress int
	obstacle, comment    string
}

func (f *fakeStudents) Get(_ context.Context, _, id uuid.UUID) (*studentusecase.Detail, error) {
	return &studentusecase.Detail{Student: entity.Student{ID: id, Name: f.name}}, nil
}
func (f *fakeStudents) SubmitWeeklySurvey(_ context.Context, _, _ uuid.UUID, m, p int, o, c string) (entity.Survey, error) {
	f.submitted = &submittedSurvey{motivation: m, progress: p, obstacle: o, comment: c}
	return entity.Survey{}, nil
}

func newTestSvc() (*Service, *fakeSender, *fakeBots, *fakeStudents) {
	snd, bots, studs := &fakeSender{}, newFakeBots(), &fakeStudents{name: "Ali"}
	return NewService(snd, bots, studs, "testbot", "test-token", zerolog.Nop()), snd, bots, studs
}

func msg(chatID int64, text string) telegram.Update {
	return telegram.Update{Message: &telegram.Message{Chat: telegram.Chat{ID: chatID}, Text: text}}
}
func cb(chatID int64, data string) telegram.Update {
	return telegram.Update{CallbackQuery: &telegram.CallbackQuery{ID: "c", Data: data, Message: &telegram.Message{Chat: telegram.Chat{ID: chatID}}}}
}

// --- tests (push-only bot) ---

func hasAppButton(kb *telegram.InlineKeyboardMarkup) bool {
	if kb == nil {
		return false
	}
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if b.WebApp != nil && b.WebApp.URL != "" {
				return true
			}
		}
	}
	return false
}

func (f *fakeSender) lastKb() *telegram.InlineKeyboardMarkup {
	if len(f.sent) == 0 {
		return nil
	}
	return f.sent[len(f.sent)-1].kb
}

func TestStart_GreetsAndOpensApp(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), msg(100, "/start"))
	if !strings.Contains(snd.last(), "Staydy") || !hasAppButton(snd.lastKb()) {
		t.Errorf("expected greeting + app button, got %q kb=%v", snd.last(), snd.lastKb())
	}
}

func TestAnyMessage_PointsToApp(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	for _, text := range []string{"/checkin", "/profil", "salom", "random"} {
		svc.HandleUpdate(context.Background(), msg(100, text))
		if !strings.Contains(snd.last(), "ilovada") || !hasAppButton(snd.lastKb()) {
			t.Errorf("%q: expected use-app nudge + button, got %q", text, snd.last())
		}
	}
}

func TestStaleCallback_AcksAndPointsToApp(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), cb(100, "m:1")) // leftover score button
	if !strings.Contains(snd.last(), "ilovada") || !hasAppButton(snd.lastKb()) {
		t.Errorf("expected use-app nudge + button on stale callback, got %q", snd.last())
	}
}

func TestRemindDueHomework_PushesAndMarksOnce(t *testing.T) {
	svc, snd, bots, _ := newTestSvc()
	aid := uuid.New()
	bots.reminders = []entity.HomeworkReminder{
		{AssignmentID: aid, Title: "Algebra 5", ChatID: 100},
		{AssignmentID: aid, Title: "Algebra 5", ChatID: 200}, // same assignment, 2nd recipient
	}
	n, err := svc.RemindDueHomework(context.Background(), uuid.New())
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v, want 2/nil", n, err)
	}
	if !strings.Contains(snd.last(), "Algebra 5") || !hasAppButton(snd.lastKb()) {
		t.Errorf("expected deadline msg + app button, got %q", snd.last())
	}
	if len(bots.marked) != 1 || bots.marked[0] != aid {
		t.Errorf("assignment should be marked exactly once, got %v", bots.marked)
	}
}

func TestWeeklyReminder_SendsToLinkedChats(t *testing.T) {
	svc, snd, bots, _ := newTestSvc()
	bots.linked = []repo.LinkedChat{{ChatID: 100}, {ChatID: 200}}
	n, err := svc.BroadcastWeeklySurvey(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("broadcast: n=%d err=%v, want 2/nil", n, err)
	}
	if !strings.Contains(snd.last(), "check-in") || !hasAppButton(snd.lastKb()) {
		t.Errorf("expected reminder + app button, got %q", snd.last())
	}
}
