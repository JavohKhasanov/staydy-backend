package entity

// Point kinds recorded in the student_points ledger.
const (
	PointAttendance     = "attendance"
	PointHomeworkSubmit = "homework_submit"
	PointHomeworkAccept = "homework_accept"
	PointCheckin        = "checkin"
	PointExam           = "exam"
	PointManual         = "manual"
	PointPurchase       = "purchase"
)

// GamificationConfig is a center's tunable XP/coin economy. XP is earned from actions and drives
// level + rating (never spent). Coins are derived from earned XP at a level-scaled rate and are
// spendable in the shop: coins = xp * (CoinBase + (level-1) * CoinStep); level = xp / LevelSize + 1.
type GamificationConfig struct {
	XPAttend      int // on-time attendance
	XPLate        int // late attendance
	XPHomeworkMax int // teacher grades homework 0..this
	XPCheckin     int // weekly check-in
	LevelSize     int // XP per level
	CoinBase      int // coins per XP at level 1
	CoinStep      int // extra coins per XP for each level above 1
}

// DefaultGamification is the standard economy a new center starts with.
func DefaultGamification() GamificationConfig {
	return GamificationConfig{
		XPAttend: 2, XPLate: 1, XPHomeworkMax: 5, XPCheckin: 2,
		LevelSize: 300, CoinBase: 4, CoinStep: 1,
	}
}

// LevelForXP is the 1-based Bosqich for a cumulative XP.
func (c GamificationConfig) LevelForXP(xp int) int {
	if c.LevelSize <= 0 {
		return 1
	}
	if xp < 0 {
		xp = 0
	}
	return xp/c.LevelSize + 1
}

// XPIntoLevel is progress within the current level (0..LevelSize-1).
func (c GamificationConfig) XPIntoLevel(xp int) int {
	if c.LevelSize <= 0 || xp < 0 {
		return 0
	}
	return xp % c.LevelSize
}

// CoinRateAtLevel is how many coins each XP is worth at a level (grows with level).
func (c GamificationConfig) CoinRateAtLevel(level int) int {
	r := c.CoinBase + (level-1)*c.CoinStep
	if r < 0 {
		r = 0
	}
	return r
}

// CoinsForAward is the coins earned for an XP award, priced at the student's current level.
func (c GamificationConfig) CoinsForAward(xpAward, currentXP int) int {
	if xpAward <= 0 {
		return 0
	}
	return xpAward * c.CoinRateAtLevel(c.LevelForXP(currentXP))
}
