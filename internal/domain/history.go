package domain

import (
	"slices"
	"time"

	"github.com/tomasbasham/gofp"
)

// CheckHistoryEntry is the outcome of one test of a check.
type CheckHistoryEntry struct {
	At       time.Time
	Location string

	// StatusCode is the HTTP status the target answered with, when there was
	// one. ResponseTime is how long it took to answer.
	StatusCode   gofp.Option[int]
	ResponseTime gofp.Option[time.Duration]
}

// CheckHistoryFilter narrows the history of one check to a window of time.
type CheckHistoryFilter struct {
	CheckRef

	Before gofp.Option[time.Time]
	After  gofp.Option[time.Time]

	// Limit is the maximum number of entries wanted. Adapters should ask for
	// one more than this, so that [NewCheckHistory] can tell whether there are
	// further entries without being given all of them.
	Limit int
}

// CheckHistory is a window onto the tests a check has run, newest first.
type CheckHistory struct {
	Entries []CheckHistoryEntry

	// More reports that entries exist beyond those listed.
	More bool
}

// NewCheckHistory orders entries newest first and keeps the first limit of
// them. More is set when that discarded anything. A limit of zero or less
// keeps everything.
func NewCheckHistory(entries []CheckHistoryEntry, limit int) CheckHistory {
	sorted := slices.Clone(entries)
	slices.SortStableFunc(sorted, func(a, b CheckHistoryEntry) int {
		return b.At.Compare(a.At)
	})

	more := false
	if limit > 0 && len(sorted) > limit {
		sorted, more = sorted[:limit], true
	}
	return CheckHistory{Entries: sorted, More: more}
}
