// Package chat is the student's AI study assistant ("Staydy Yordamchi") — a friendly Gemini-backed
// helper for motivation, study tips, planning, and exam prep. It nudges and hints; it never does the
// homework for the student.
package chat

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Generator is the model port (satisfied by platform/gemini.Client).
type Generator interface {
	Generate(ctx context.Context, system, prompt string) (string, error)
	Configured() bool
}

// Turn is one prior message in the conversation.
type Turn struct {
	Role string // "user" | "assistant"
	Text string
}

// maxHistory bounds how much prior context we replay into the prompt.
const maxHistory = 12

const fallback = "Kechirasiz, AI yordamchi hozircha mavjud emas. Savolingizni o'qituvchingizga yo'llashingiz mumkin. 🙏"

func systemInstruction(name string) string {
	who := "talaba"
	if strings.TrimSpace(name) != "" {
		who = name
	}
	return "Sen \"Staydy Yordamchi\"san — o'quv markazidagi " + who + "ga yordam beradigan do'stona AI. " +
		"Vazifang: motivatsiya berish, o'qish bo'yicha maslahat, vaqtni rejalashtirish, imtihonga tayyorlanish va qiyin mavzularni oddiy tushuntirish. " +
		"Uy vazifasini talaba O'RNIGA yechib BERMA — yo'l ko'rsat, ipuchi ber, o'ylashga undab. " +
		"Qisqa (2-4 gap), iliq va o'zbek tilida javob ber. Hurmatli, ruhlantiruvchi ohangda."
}

type Service struct {
	gen Generator
}

func NewService(gen Generator) *Service {
	return &Service{gen: gen}
}

// Reply answers the student's message given recent history. Returns a graceful fallback (never an
// error to the caller) when the model isn't configured or fails.
func (s *Service) Reply(ctx context.Context, _ uuid.UUID, studentName, message string, history []Turn) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "Savolingizni yozing 🙂"
	}
	if s.gen == nil || !s.gen.Configured() {
		return fallback
	}

	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}
	var b strings.Builder
	for _, t := range history {
		role := "Talaba"
		if t.Role == "assistant" {
			role = "Yordamchi"
		}
		b.WriteString(role + ": " + strings.TrimSpace(t.Text) + "\n")
	}
	b.WriteString("Talaba: " + message + "\nYordamchi:")

	text, err := s.gen.Generate(ctx, systemInstruction(studentName), b.String())
	if err != nil || strings.TrimSpace(text) == "" {
		return fallback
	}
	return strings.TrimSpace(text)
}
