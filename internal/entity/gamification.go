package entity

// Point kinds recorded in the student_points ledger.
const (
	PointAttendance     = "attendance"
	PointHomeworkSubmit = "homework_submit"
	PointHomeworkAccept = "homework_accept"
	PointCheckin        = "checkin"
	PointPurchase       = "purchase"
	PointManual         = "manual"
)

// XP / coin rewards per action (tunable). A student earns these automatically as they attend, do
// homework, and check in — the values behind the mini app's momentum meter and leaderboard.
const (
	XPAttendance    = 10
	CoinsAttendance = 5

	XPHomeworkOnTime    = 20
	CoinsHomeworkOnTime = 10
	XPHomeworkLate      = 10 // late submission still earns some XP, no coins

	CoinsHomeworkAccept = 5 // XP for an accepted submission is the graded score itself

	XPCheckin    = 15
	CoinsCheckin = 10
)

// XPPerLevel is the XP needed to advance one Bosqich (level). Flat for now; tune later.
const XPPerLevel = 1000

// Level is the student's current Bosqich (1-based) for a total XP.
func Level(xp int) int {
	if xp < 0 {
		xp = 0
	}
	return xp/XPPerLevel + 1
}

// XPIntoLevel is progress within the current level (0..XPPerLevel-1).
func XPIntoLevel(xp int) int {
	if xp < 0 {
		return 0
	}
	return xp % XPPerLevel
}
