package student

import (
	"strings"
	"testing"

	"github.com/student-success/backend/internal/entity"
)

func reasonsContain(reasons []string, sub string) bool {
	for _, r := range reasons {
		if strings.Contains(r, sub) {
			return true
		}
	}
	return false
}

// >= triggerConsecutiveAbsences trailing absences force Red even when the raw score is Yellow.
func TestOutcome_ConsecutiveAbsenceForcesRed(t *testing.T) {
	svc := newTestSvc(&fakeRepo{})
	att := []entity.AttendanceRecord{
		{IsPresent: true}, {IsPresent: true}, {IsPresent: true},
		{IsPresent: true}, {IsPresent: true}, {IsPresent: true},
		{IsPresent: false}, {IsPresent: false}, {IsPresent: false}, {IsPresent: false},
	} // 60% attendance (raw score Yellow), last 4 absent
	out := svc.outcome(entity.Student{ConfidenceLevel: 8}, att, nil, nil)
	if out.Tier != "Red" {
		t.Fatalf("tier = %s, want Red (forced by consecutive absences)", out.Tier)
	}
	if !reasonsContain(out.TaskReasons, "Ketma-ket") {
		t.Fatalf("reasons %v missing the consecutive-absence trigger", out.TaskReasons)
	}
}

// A >= triggerScoreJump increase vs the previously stored score opens a task even at Yellow.
func TestOutcome_ScoreJumpOpensTaskAtYellow(t *testing.T) {
	svc := newTestSvc(&fakeRepo{})
	att := []entity.AttendanceRecord{
		{IsPresent: true}, {IsPresent: true}, {IsPresent: true},
		{IsPresent: false}, {IsPresent: false}, // 60%, only 2 trailing absences (no absence trigger)
	}
	prev := entity.Student{ConfidenceLevel: 8, RiskScore: 5} // previously stored score = 5
	out := svc.outcome(prev, att, nil, nil)
	if out.Tier != "Yellow" {
		t.Fatalf("tier = %s, want Yellow", out.Tier)
	}
	if !reasonsContain(out.TaskReasons, "Xavf keskin oshdi") {
		t.Fatalf("reasons %v missing the score-jump trigger", out.TaskReasons)
	}
}
