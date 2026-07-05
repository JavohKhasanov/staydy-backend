// Package risk implements the student churn-risk scoring engine.
//
// It is a pure function — no database, no I/O — so it is fully unit-testable and
// is the single source of truth for risk_score and risk_tier. Ported from the
// frontend's calculateStudentRisk. Evaluate also returns a Factor breakdown so
// the UI can show an accurate "why" instead of hardcoded labels.
package risk

type Tier string

const (
	TierGreen  Tier = "Green"
	TierYellow Tier = "Yellow"
	TierRed    Tier = "Red"
)

// Tier thresholds on the 0..100 score (higher = more at risk).
const (
	TierThresholdRed    = 70 // score >= 70 → Red
	TierThresholdYellow = 35 // score >= 35 → Yellow
)

// Inputs is the minimal signal set needed to score a student.
type Inputs struct {
	AttendanceRate   float64  // 0..100
	HomeworkRate     *float64 // 0..100, nil if no homework recorded yet (neutral)
	LatestMotivation *int     // 1..5, nil if no survey yet
	LatestProgress   *int     // 1..5, nil if no survey yet
	ConfidenceLevel  int      // 1..10 (from onboarding)
}

// Factor explains one contribution to the total score. Neutral marks a "missing data"
// placeholder (e.g. no survey submitted yet) — it adds points but is NOT a churn driver,
// so callers exclude it from intervention-task reasons.
type Factor struct {
	Label   string `json:"label"`
	Points  int    `json:"points"`
	Neutral bool   `json:"neutral,omitempty"`
}

// Result is the scored outcome.
type Result struct {
	Score   int      `json:"score"` // 0..100
	Tier    Tier     `json:"tier"`
	Factors []Factor `json:"factors"`
}

// Evaluate computes the risk score, tier, and factor breakdown.
func Evaluate(in Inputs) Result {
	factors := make([]Factor, 0, 4)
	score := 0
	addN := func(label string, pts int, neutral bool) {
		score += pts
		factors = append(factors, Factor{Label: label, Points: pts, Neutral: neutral})
	}
	add := func(label string, pts int) { addN(label, pts, false) }

	// 1. Attendance (max 40).
	switch {
	case in.AttendanceRate < 50:
		add("Davomat juda past (<50%)", 40)
	case in.AttendanceRate < 70:
		add("Davomat past (<70%)", 30)
	case in.AttendanceRate < 80:
		add("Davomat o'rtacha (<80%)", 20)
	case in.AttendanceRate < 90:
		add("Davomat biroz past (<90%)", 10)
	}

	// 1b. Homework completion (max 20); nil → nothing assigned yet, neutral (no points).
	if in.HomeworkRate != nil {
		switch {
		case *in.HomeworkRate < 50:
			add("Uy vazifa juda past (<50%)", 20)
		case *in.HomeworkRate < 70:
			add("Uy vazifa past (<70%)", 15)
		case *in.HomeworkRate < 80:
			add("Uy vazifa o'rtacha (<80%)", 10)
		case *in.HomeworkRate < 90:
			add("Uy vazifa biroz past (<90%)", 5)
		}
	}

	// 2. Latest motivation (max 25); neutral +10 when no survey yet.
	if in.LatestMotivation == nil {
		addN("So'rovnoma topshirilmagan", 10, true) // neutral: missing data, not a churn driver
	} else {
		switch *in.LatestMotivation {
		case 1:
			add("Motivatsiya juda past (1/5)", 25)
		case 2:
			add("Motivatsiya past (2/5)", 20)
		case 3:
			add("Motivatsiya o'rtacha (3/5)", 12)
		case 4:
			add("Motivatsiya yaxshi (4/5)", 5)
		}
	}

	// 3. Latest progress sense (max 20).
	if in.LatestProgress != nil {
		switch *in.LatestProgress {
		case 1:
			add("Progress hissi yo'q (1/5)", 20)
		case 2:
			add("Progress hissi past (2/5)", 15)
		case 3:
			add("Progress hissi o'rtacha (3/5)", 10)
		case 4:
			add("Progress hissi yaxshi (4/5)", 4)
		}
	}

	// 4. Onboarding confidence (max 15).
	switch {
	case in.ConfidenceLevel <= 3:
		add("Maqsadga ishonch past (<=3)", 15)
	case in.ConfidenceLevel <= 6:
		add("Maqsadga ishonch o'rtacha (<=6)", 10)
	case in.ConfidenceLevel <= 8:
		add("Maqsadga ishonch yaxshi (<=8)", 5)
	}

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	tier := TierGreen
	switch {
	case score >= TierThresholdRed:
		tier = TierRed
	case score >= TierThresholdYellow:
		tier = TierYellow
	}

	return Result{Score: score, Tier: tier, Factors: factors}
}
