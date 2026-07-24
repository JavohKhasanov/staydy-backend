package entity

// RetentionStats is the director-facing retention picture: who's still here, the risk mix, how each
// join-cohort has held up, and how effective interventions have been.
type RetentionStats struct {
	Total         int     `json:"total"`   // non-lead students
	Active        int     `json:"active"`
	Dropped       int     `json:"dropped"`
	RetentionRate float64 `json:"retentionRate"` // active / (active + dropped), 0..1
	Green         int     `json:"green"`
	Yellow        int     `json:"yellow"`
	Red           int     `json:"red"`
	Cohorts       []Cohort          `json:"cohorts"`
	Interventions InterventionStats `json:"interventions"`
}

// Cohort is one join-month's retention.
type Cohort struct {
	Month         string  `json:"month"` // YYYY-MM
	Total         int     `json:"total"`
	Active        int     `json:"active"`
	Dropped       int     `json:"dropped"`
	RetentionRate float64 `json:"retentionRate"`
}

// InterventionStats measures the follow-up loop.
type InterventionStats struct {
	Open           int     `json:"open"`
	Resolved       int     `json:"resolved"`
	Resolved30d    int     `json:"resolved30d"`
	AvgResolveDays float64 `json:"avgResolveDays"`
}

// retentionRate is active/(active+dropped); 1.0 when there are no departures yet.
func retentionRate(active, dropped int) float64 {
	base := active + dropped
	if base == 0 {
		return 1
	}
	return float64(active) / float64(base)
}

// ComputeRetention fills the derived retention rates from the raw counts.
func (s *RetentionStats) ComputeRetention() {
	s.RetentionRate = retentionRate(s.Active, s.Dropped)
	for i := range s.Cohorts {
		s.Cohorts[i].RetentionRate = retentionRate(s.Cohorts[i].Active, s.Cohorts[i].Dropped)
	}
}
