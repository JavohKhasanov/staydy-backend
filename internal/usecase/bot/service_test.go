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
	invites map[string]repo.BotInvite
	convos  map[int64]repo.BotConversation
	used    map[string]bool
	chatSet map[uuid.UUID]int64
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
func (f *fakeBots) ListLinkedChats(context.Context) ([]repo.LinkedChat, error) { return nil, nil }
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

// --- tests ---

func TestStart_NoToken_NotLinked(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), msg(100, "/start"))
	if !strings.Contains(snd.last(), "ulanmagansiz") {
		t.Errorf("expected not-linked message, got %q", snd.last())
	}
}

func TestProfile_ShowsZoneAndName(t *testing.T) {
	svc, snd, bots, _ := newTestSvc()
	_ = bots.BindChat(context.Background(), 200, uuid.New(), uuid.New()) // link the chat
	svc.HandleUpdate(context.Background(), msg(200, "/profil"))
	out := snd.last()
	if !strings.Contains(out, "Ali") || !strings.Contains(out, "zona") {
		t.Errorf("expected profile with name + zone, got %q", out)
	}
}

func TestProfile_NotLinked(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), msg(201, "/profil"))
	if !strings.Contains(snd.last(), "ulanmagansiz") {
		t.Errorf("expected not-linked, got %q", snd.last())
	}
}

func TestStart_BadToken(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), msg(100, "/start nope"))
	if !strings.Contains(snd.last(), "noto'g'ri") {
		t.Errorf("expected bad-token message, got %q", snd.last())
	}
}

func TestFullCheckinFlow_SubmitsSurvey(t *testing.T) {
	svc, snd, bots, studs := newTestSvc()
	ctx := context.Background()
	orgID, studentID := uuid.New(), uuid.New()
	_ = bots.CreateInvite(ctx, "tok", orgID, studentID, uuid.Nil, time.Now().Add(time.Hour))

	// 1. link via deep link → binds chat, marks token used, sets student chat, then prompts motivation
	svc.HandleUpdate(ctx, msg(100, "/start tok"))
	if !bots.used["tok"] {
		t.Fatal("token should be marked used")
	}
	if bots.chatSet[studentID] != 100 {
		t.Fatal("student chat id should be bound")
	}
	if !strings.Contains(snd.last(), "motivatsiya") {
		t.Fatalf("expected motivation prompt, got %q", snd.last())
	}

	// 2. walk the inline-keyboard FSM
	svc.HandleUpdate(ctx, cb(100, "m:1"))
	if !strings.Contains(snd.last(), "Progress") {
		t.Fatalf("expected progress prompt, got %q", snd.last())
	}
	svc.HandleUpdate(ctx, cb(100, "p:2"))
	if !strings.Contains(snd.last(), "to'sig'ingiz") {
		t.Fatalf("expected obstacle prompt, got %q", snd.last())
	}
	svc.HandleUpdate(ctx, cb(100, "o:1")) // "Ish"
	if !strings.Contains(snd.last(), "izoh") {
		t.Fatalf("expected comment prompt, got %q", snd.last())
	}

	// 3. comment text finalizes → survey submitted with the accumulated answers
	svc.HandleUpdate(ctx, msg(100, "yordam kerak"))
	if studs.submitted == nil {
		t.Fatal("survey should have been submitted")
	}
	got := studs.submitted
	if got.motivation != 1 || got.progress != 2 || got.obstacle != "Ish" || got.comment != "yordam kerak" {
		t.Errorf("submitted survey wrong: %+v", got)
	}
	if !strings.Contains(snd.last(), "qabul qilindi") {
		t.Errorf("expected done message, got %q", snd.last())
	}
	// flow cleared
	if c := bots.convos[100]; c.Flow != "" {
		t.Errorf("flow should be cleared, got %q", c.Flow)
	}
}

func TestSkipComment_SubmitsEmptyComment(t *testing.T) {
	svc, _, bots, studs := newTestSvc()
	ctx := context.Background()
	orgID, studentID := uuid.New(), uuid.New()
	_ = bots.CreateInvite(ctx, "tok", orgID, studentID, uuid.Nil, time.Now().Add(time.Hour))
	svc.HandleUpdate(ctx, msg(100, "/start tok"))
	svc.HandleUpdate(ctx, cb(100, "m:3"))
	svc.HandleUpdate(ctx, cb(100, "p:3"))
	svc.HandleUpdate(ctx, cb(100, "o:0"))
	svc.HandleUpdate(ctx, msg(100, "/skip"))
	if studs.submitted == nil || studs.submitted.comment != "" {
		t.Errorf("expected empty comment on /skip, got %+v", studs.submitted)
	}
}

func TestUsedToken_Rejected(t *testing.T) {
	svc, snd, bots, _ := newTestSvc()
	ctx := context.Background()
	_ = bots.CreateInvite(ctx, "tok", uuid.New(), uuid.New(), uuid.Nil, time.Now().Add(time.Hour))
	bots.used["tok"] = true
	svc.HandleUpdate(ctx, msg(100, "/start tok"))
	if !strings.Contains(snd.last(), "ishlatilgan") {
		t.Errorf("expected used-token message, got %q", snd.last())
	}
}

func TestCheckin_NotLinked_Rejected(t *testing.T) {
	svc, snd, _, _ := newTestSvc()
	svc.HandleUpdate(context.Background(), msg(100, "/checkin"))
	if !strings.Contains(snd.last(), "ulanmagansiz") {
		t.Errorf("expected not-linked message, got %q", snd.last())
	}
}
