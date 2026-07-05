// Package advice is the AI-advisor usecase: it turns a student's risk signals into a short,
// actionable recommendation for the mentor (in Uzbek) via a generative model. The model call is
// behind a Generator port so the usecase is testable without network access.
package advice

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/student-success/backend/internal/entity"
	"github.com/student-success/backend/internal/risk"
)

// ErrNotConfigured is returned when no model backend is available (e.g. missing API key).
var ErrNotConfigured = errors.New("advice: AI not configured")

// Generator is the model port (satisfied by platform/gemini.Client).
type Generator interface {
	Generate(ctx context.Context, system, prompt string) (string, error)
	Configured() bool
}

// Input is everything the advisor reasons over (built by the controller from the student detail).
type Input struct {
	Student    entity.Student
	Surveys    []entity.Survey
	Attendance []entity.AttendanceRecord
	Notes      []entity.Note
	Factors    []risk.Factor
}

// Advice is the generated recommendation.
type Advice struct {
	Text        string
	GeneratedAt time.Time
	Cached      bool
}

type cacheEntry struct {
	hash string
	text string
	at   time.Time
}

type Service struct {
	gen Generator
	ttl time.Duration

	mu    sync.Mutex
	cache map[uuid.UUID]cacheEntry
}

// maxCacheEntries triggers a sweep of expired entries once the cache grows past it.
const maxCacheEntries = 1000

func NewService(gen Generator) *Service {
	return &Service{gen: gen, ttl: 15 * time.Minute, cache: make(map[uuid.UUID]cacheEntry)}
}

const systemInstruction = "Sen o'quv markazidagi talabalarni o'qishni tashlab ketishdan saqlab " +
	"qolish bo'yicha tajribali maslahatchisan. Mentorga qisqa, aniq va amaliy tavsiyalar berasan. " +
	"Faqat o'zbek tilida, samimiy va hurmatli ohangda javob ber. Markdown formatidan foydalan."

// Advise returns a recommendation for the student, regenerating only when the inputs change or the
// cached copy is stale (an in-process TTL cache — fine for a single instance; not shared across replicas).
func (s *Service) Advise(ctx context.Context, in Input) (Advice, error) {
	if s.gen == nil || !s.gen.Configured() {
		return Advice{}, ErrNotConfigured
	}

	h := inputHash(in)
	s.mu.Lock()
	if e, ok := s.cache[in.Student.ID]; ok && e.hash == h && time.Since(e.at) < s.ttl {
		text, at := e.text, e.at
		s.mu.Unlock()
		return Advice{Text: text, GeneratedAt: at, Cached: true}, nil
	}
	s.mu.Unlock()

	text, err := s.gen.Generate(ctx, systemInstruction, buildPrompt(in))
	if err != nil {
		if errors.Is(err, ErrNotConfigured) {
			return Advice{}, ErrNotConfigured
		}
		return Advice{}, err
	}

	now := time.Now()
	s.mu.Lock()
	s.cache[in.Student.ID] = cacheEntry{hash: h, text: text, at: now}
	// Bound growth: when the map gets large, drop entries past their TTL (e.g. students no longer
	// being advised) so it doesn't accumulate every student ever advised.
	if len(s.cache) > maxCacheEntries {
		for id, e := range s.cache {
			if now.Sub(e.at) >= s.ttl {
				delete(s.cache, id)
			}
		}
	}
	s.mu.Unlock()
	return Advice{Text: text, GeneratedAt: now, Cached: false}, nil
}

// attendanceRate returns present/total as a percentage (100 when there are no records — neutral).
func attendanceRate(att []entity.AttendanceRecord) int {
	if len(att) == 0 {
		return 100
	}
	present := 0
	for _, a := range att {
		if a.IsPresent {
			present++
		}
	}
	return present * 100 / len(att)
}

func latestSurvey(surveys []entity.Survey) (entity.Survey, bool) {
	if len(surveys) == 0 {
		return entity.Survey{}, false
	}
	latest := surveys[0]
	for _, sv := range surveys[1:] {
		if sv.WeekNumber > latest.WeekNumber {
			latest = sv
		}
	}
	return latest, true
}

func buildPrompt(in Input) string {
	st := in.Student
	var b strings.Builder
	b.WriteString("Quyidagi talaba haqida ma'lumotlar:\n")
	fmt.Fprintf(&b, "- Ism: %s\n", fallback(st.Name, "noma'lum"))
	fmt.Fprintf(&b, "- Kurs: %s, Guruh: %s\n", fallback(st.CourseName, "—"), fallback(st.GroupName, "—"))
	fmt.Fprintf(&b, "- Mentor: %s\n", fallback(st.MentorName, "—"))
	if st.OnboardingGoal != "" {
		fmt.Fprintf(&b, "- Maqsadi: %s\n", st.OnboardingGoal)
	}
	if st.SixMonthTarget != "" {
		fmt.Fprintf(&b, "- 6 oylik maqsad: %s\n", st.SixMonthTarget)
	}
	fmt.Fprintf(&b, "- Boshlang'ich ishonch darajasi (1-10): %d\n", st.ConfidenceLevel)
	fmt.Fprintf(&b, "- Davomat: %d%%\n", attendanceRate(in.Attendance))
	fmt.Fprintf(&b, "- Risk balli: %d/100 (%s daraja)\n", st.RiskScore, riskTierUz(st.RiskTier))

	if sv, ok := latestSurvey(in.Surveys); ok {
		fmt.Fprintf(&b, "- Oxirgi so'rovnoma (%d-hafta): motivatsiya %d/5, progress hissi %d/5",
			sv.WeekNumber, sv.MotivationScore, sv.ProgressScore)
		if sv.BiggestObstacle != "" {
			fmt.Fprintf(&b, ", asosiy to'siq: %s", sv.BiggestObstacle)
		}
		b.WriteString("\n")
		if strings.TrimSpace(sv.Comment) != "" {
			fmt.Fprintf(&b, "- Talaba izohi: %q\n", sv.Comment)
		}
	} else {
		b.WriteString("- Talaba hali so'rovnoma topshirmagan.\n")
	}

	if labels := factorLabels(in.Factors); len(labels) > 0 {
		fmt.Fprintf(&b, "- Risk omillari: %s\n", strings.Join(labels, "; "))
	}

	b.WriteString("\nMentor uchun quyidagi 3 bo'limda tavsiya yoz:\n")
	b.WriteString("1. **Risk sabablari** — talaba nega xavf ostida ekanini 2-3 jumlada tushuntir.\n")
	b.WriteString("2. **Tavsiya etilgan harakatlar** — mentor bajaradigan 2-3 ta aniq qadam.\n")
	b.WriteString("3. **Suhbatni boshlash** — talabaga Telegram orqali yuborish mumkin bo'lgan samimiy xabar namunasi.\n")
	return b.String()
}

func factorLabels(factors []risk.Factor) []string {
	out := make([]string, 0, len(factors))
	for _, f := range factors {
		if f.Neutral || f.Points <= 0 {
			continue
		}
		out = append(out, f.Label)
	}
	return out
}

func riskTierUz(tier string) string {
	switch tier {
	case "Red":
		return "Qizil (yuqori)"
	case "Yellow":
		return "Sariq (o'rta)"
	case "Green":
		return "Yashil (past)"
	default:
		return tier
	}
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// inputHash fingerprints the signals that actually drive the prompt, so the advice regenerates
// when they change but not on an identical re-click. Notes are intentionally excluded — they're
// not part of buildPrompt, so they must not invalidate the cache. A delimiter between fields
// prevents boundary ambiguity (e.g. "12"+"3" vs "1"+"23").
func inputHash(in Input) string {
	h := fnv.New64a()
	write := func(s string) {
		_, _ = h.Write([]byte(s))
		_, _ = h.Write([]byte{0})
	}
	st := in.Student
	write(st.RiskTier)
	write(strconv.Itoa(st.RiskScore))
	write(strconv.Itoa(attendanceRate(in.Attendance)))
	write(strconv.Itoa(len(in.Attendance)))
	if sv, ok := latestSurvey(in.Surveys); ok {
		write(sv.SubmittedAt.UTC().Format(time.RFC3339Nano))
		write(strconv.Itoa(sv.MotivationScore))
		write(strconv.Itoa(sv.ProgressScore))
		write(sv.BiggestObstacle)
	}
	return strconv.FormatUint(h.Sum64(), 16)
}
