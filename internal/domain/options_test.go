package domain_test

import (
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tomasbasham/gofp"
)

// cmpOptions teaches go-cmp to compare the Options that appear in domain
// values. An Option holds only unexported fields, so cmp cannot look inside
// one, but it is comparable.
func cmpOptions() cmp.Option {
	return cmpopts.EquateComparable(
		gofp.Option[int]{},
		gofp.Option[string]{},
		gofp.Option[time.Duration]{},
		gofp.Option[time.Time]{},
	)
}
