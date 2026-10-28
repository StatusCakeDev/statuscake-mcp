package domain

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tomasbasham/gofp"
)

// MaintenanceState is where a maintenance window is in its life cycle.
type MaintenanceState string

const (
	MaintenanceStateActive  MaintenanceState = "active"
	MaintenanceStatePending MaintenanceState = "pending"
	MaintenanceStatePaused  MaintenanceState = "paused"
)

var maintenanceStates = []MaintenanceState{
	MaintenanceStateActive,
	MaintenanceStatePending,
	MaintenanceStatePaused,
}

// String implements [fmt.Stringer].
func (s MaintenanceState) String() string { return string(s) }

// ParseMaintenanceState validates a raw string. See [ParseCheckType].
func ParseMaintenanceState(raw string) (MaintenanceState, error) {
	s := MaintenanceState(strings.ToLower(strings.TrimSpace(raw)))
	if !slices.Contains(maintenanceStates, s) {
		return "", fmt.Errorf("invalid maintenance state %q: must be one of %s", raw, oneOf(maintenanceStates))
	}
	return s, nil
}

// triageRank puts windows that are suppressing alerts right now first.
func (s MaintenanceState) triageRank() int {
	switch s {
	case MaintenanceStateActive:
		return 0
	case MaintenanceStatePending:
		return 1
	default:
		return 2
	}
}

// MaintenanceWindow is a period during which alerts for some checks are
// suppressed.
type MaintenanceWindow struct {
	ID    string
	Name  string
	State MaintenanceState

	Start time.Time
	End   time.Time

	// Repeat describes how often the window recurs, such as "never" or
	// "weekly".
	Repeat   string
	Timezone string

	// Tags select checks by tag. Checks select checks by ID. A window covers a
	// check if either applies.
	Tags   []string
	Checks []string
}

// MaintenanceWindowFilter narrows a listing.
type MaintenanceWindowFilter struct {
	State gofp.Option[MaintenanceState]

	// Limit is the maximum number of windows wanted. Adapters should pass every
	// match to [NewMaintenanceWindowList], which applies it.
	Limit int
}

// MaintenanceWindowList is one page of maintenance windows.
type MaintenanceWindowList struct {
	// Windows holds at most the requested limit: active windows first, then
	// pending, then paused, each ordered by start time.
	Windows []MaintenanceWindow

	// Total is the number of windows that matched, before the limit.
	Total int
}

// NewMaintenanceWindowList orders windows by relevance and keeps the first
// limit of them. A limit of zero or less keeps everything.
func NewMaintenanceWindowList(windows []MaintenanceWindow, limit int) MaintenanceWindowList {
	sorted := slices.Clone(windows)
	slices.SortStableFunc(sorted, func(a, b MaintenanceWindow) int {
		return cmp.Or(
			cmp.Compare(a.State.triageRank(), b.State.triageRank()),
			a.Start.Compare(b.Start),
			strings.Compare(a.ID, b.ID),
		)
	})

	total := len(sorted)
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return MaintenanceWindowList{Windows: sorted, Total: total}
}

// Truncated reports whether more windows matched than the list holds.
func (l MaintenanceWindowList) Truncated() bool {
	return l.Total > len(l.Windows)
}
