package risk

import "testing"

func ptr(i int) *int { return &i }

func TestEvaluate_RedWhenLowAttendanceAndMotivation(t *testing.T) {
	got := Evaluate(Inputs{
		AttendanceRate:   45,      // +40
		LatestMotivation: ptr(2),  // +20
		LatestProgress:   ptr(2),  // +15
		ConfidenceLevel:  5,       // +10
	})
	if got.Score != 85 {
		t.Fatalf("score = %d, want 85", got.Score)
	}
	if got.Tier != TierRed {
		t.Fatalf("tier = %s, want Red", got.Tier)
	}
}

func TestEvaluate_GreenHealthyStudent(t *testing.T) {
	got := Evaluate(Inputs{
		AttendanceRate:   100,
		LatestMotivation: ptr(5),
		LatestProgress:   ptr(5),
		ConfidenceLevel:  9,
	})
	if got.Score != 0 {
		t.Fatalf("score = %d, want 0", got.Score)
	}
	if got.Tier != TierGreen {
		t.Fatalf("tier = %s, want Green", got.Tier)
	}
}

func TestEvaluate_NoSurveyAddsNeutralMotivation(t *testing.T) {
	got := Evaluate(Inputs{AttendanceRate: 100, ConfidenceLevel: 10})
	if got.Score != 10 {
		t.Fatalf("score = %d, want 10", got.Score)
	}
	if got.Tier != TierGreen {
		t.Fatalf("tier = %s, want Green", got.Tier)
	}
}
