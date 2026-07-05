package risk

import "testing"

func TestEvaluate_HomeworkFactor(t *testing.T) {
	m5, p5 := 5, 5 // motivation/progress 5 → 0 points (isolate the homework factor)
	low := 40.0
	got := Evaluate(Inputs{AttendanceRate: 100, HomeworkRate: &low, LatestMotivation: &m5, LatestProgress: &p5, ConfidenceLevel: 10})
	if got.Score != 20 {
		t.Fatalf("homework 40%%: score = %d, want 20", got.Score)
	}
	// nil homework is neutral (no points), unlike a 0% rate.
	none := Evaluate(Inputs{AttendanceRate: 100, LatestMotivation: &m5, LatestProgress: &p5, ConfidenceLevel: 10})
	if none.Score != 0 {
		t.Fatalf("no homework: score = %d, want 0", none.Score)
	}
}
