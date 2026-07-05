package advice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/risk"
)

type fakeGen struct {
	configured bool
	calls      int
	lastPrompt string
	resp       string
	err        error
}

func (f *fakeGen) Configured() bool { return f.configured }
func (f *fakeGen) Generate(_ context.Context, _, prompt string) (string, error) {
	f.calls++
	f.lastPrompt = prompt
	return f.resp, f.err
}

func sampleInput() Input {
	return Input{
		Student: entity.Student{
			ID: uuid.New(), Name: "Ali Valiyev", CourseName: "Go", GroupName: "FS-101",
			ConfidenceLevel: 3, RiskScore: 85, RiskTier: "Red",
		},
		Attendance: []entity.AttendanceRecord{{IsPresent: false}, {IsPresent: true}},
		Surveys: []entity.Survey{
			{WeekNumber: 1, MotivationScore: 3, ProgressScore: 3, SubmittedAt: time.Unix(1000, 0)},
			{WeekNumber: 2, MotivationScore: 1, ProgressScore: 2, BiggestObstacle: "Ish", SubmittedAt: time.Unix(2000, 0)},
		},
		Factors: []risk.Factor{
			{Label: "Davomat past", Points: 30},
			{Label: "So'rovnoma topshirilmagan", Points: 10, Neutral: true},
		},
	}
}

func TestAdvise_NotConfigured(t *testing.T) {
	svc := NewService(&fakeGen{configured: false})
	if _, err := svc.Advise(context.Background(), sampleInput()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
}

func TestAdvise_GeneratesThenCaches(t *testing.T) {
	gen := &fakeGen{configured: true, resp: "tavsiya matni"}
	svc := NewService(gen)
	in := sampleInput()

	a1, err := svc.Advise(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a1.Text != "tavsiya matni" || a1.Cached {
		t.Errorf("first call: got %+v, want fresh text", a1)
	}
	// Same input → served from cache, generator not called again.
	a2, err := svc.Advise(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a2.Cached {
		t.Error("second identical call should be cached")
	}
	if gen.calls != 1 {
		t.Errorf("generator called %d times, want 1 (cache hit)", gen.calls)
	}
}

func TestAdvise_RegeneratesWhenInputChanges(t *testing.T) {
	gen := &fakeGen{configured: true, resp: "x"}
	svc := NewService(gen)
	in := sampleInput()
	if _, err := svc.Advise(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	// A new survey lands → hash changes → must regenerate.
	in.Surveys = append(in.Surveys, entity.Survey{WeekNumber: 3, MotivationScore: 2, SubmittedAt: time.Unix(3000, 0)})
	a, err := svc.Advise(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Cached {
		t.Error("changed input should not be cached")
	}
	if gen.calls != 2 {
		t.Errorf("generator called %d times, want 2", gen.calls)
	}
}

func TestBuildPrompt_IncludesKeySignals(t *testing.T) {
	p := buildPrompt(sampleInput())
	for _, want := range []string{"Ali Valiyev", "Davomat: 50%", "Risk balli: 85", "2-hafta", "Ish", "Davomat past"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q\n---\n%s", want, p)
		}
	}
	// Neutral factor must NOT be presented as a risk driver.
	if strings.Contains(p, "So'rovnoma topshirilmagan") {
		t.Error("neutral factor should be excluded from prompt risk factors")
	}
}

func TestAdvise_PropagatesGeneratorError(t *testing.T) {
	wantErr := errors.New("boom")
	svc := NewService(&fakeGen{configured: true, err: wantErr})
	if _, err := svc.Advise(context.Background(), sampleInput()); !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
