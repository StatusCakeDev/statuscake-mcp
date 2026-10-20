package domain

import (
	"cmp"
	"slices"
	"strings"
)

// ContactGroup is a set of people and services that are alerted together.
//
// Only the size of each kind of recipient is modelled. Addresses and numbers
// are personal data, and a model asked "who gets alerted?" does not need them.
type ContactGroup struct {
	ID   string
	Name string

	Emails       int
	Mobiles      int
	Integrations int
}

// ContactGroupFilter bounds a listing.
type ContactGroupFilter struct {
	// Limit is the maximum number of groups wanted. Adapters should pass every
	// group to [NewContactGroupList], which applies it.
	Limit int
}

// ContactGroupList is one page of contact groups.
type ContactGroupList struct {
	// Groups holds at most the requested limit, ordered by name.
	Groups []ContactGroup

	// Total is the number of groups that exist, before the limit.
	Total int
}

// NewContactGroupList orders groups by name and keeps the first limit of
// them. A limit of zero or less keeps everything.
func NewContactGroupList(groups []ContactGroup, limit int) ContactGroupList {
	sorted := slices.Clone(groups)
	slices.SortStableFunc(sorted, func(a, b ContactGroup) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			strings.Compare(a.ID, b.ID),
		)
	})

	total := len(sorted)
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return ContactGroupList{Groups: sorted, Total: total}
}

// Truncated reports whether more groups exist than the list holds.
func (l ContactGroupList) Truncated() bool {
	return l.Total > len(l.Groups)
}
