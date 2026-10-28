package domain

import (
	"slices"
	"time"

	"github.com/tomasbasham/gofp"
)

// Incident is a period during which a check was down.
type Incident struct {
	Started time.Time

	// Ended is None while the incident is ongoing.
	Ended gofp.Option[time.Time]
}

// Ongoing reports whether the check is still down.
func (i Incident) Ongoing() bool {
	return i.Ended.IsNone()
}

// Duration is how long the check was down. For an ongoing incident it is the
// time elapsed up to now.
func (i Incident) Duration(now time.Time) time.Duration {
	return i.Ended.UnwrapOr(now).Sub(i.Started)
}

// IncidentFilter narrows the incidents of one check to a window of time.
type IncidentFilter struct {
	CheckRef

	Before gofp.Option[time.Time]
	After  gofp.Option[time.Time]

	// Limit is the maximum number of incidents wanted. Adapters should pass
	// every incident they found to [NewIncidentList], which applies it.
	Limit int
}

// IncidentList is a window onto the incidents of a check, newest first.
type IncidentList struct {
	Incidents []Incident

	// More reports that incidents exist beyond those listed.
	More bool
}

// NewIncidentList orders incidents newest first and keeps the first limit of
// them. More is set when that discarded anything, or when the adapter knows
// that its source held further pages it did not read. A limit of zero or less
// keeps everything.
func NewIncidentList(matches []Incident, limit int, more bool) IncidentList {
	sorted := slices.Clone(matches)
	slices.SortStableFunc(sorted, func(a, b Incident) int {
		return b.Started.Compare(a.Started)
	})

	if limit > 0 && len(sorted) > limit {
		sorted, more = sorted[:limit], true
	}
	return IncidentList{Incidents: sorted, More: more}
}
