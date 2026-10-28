package domain

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/tomasbasham/gofp"
)

// ErrNotFound reports that a requested resource does not exist. Adapters return
// it, or wrap it, so that handlers can give the model an actionable message
// without knowing anything about the SDK's own error types.
var ErrNotFound = errors.New("not found")

// ErrUnauthorized reports that StatusCake rejected the API token, either because
// it is not valid or because it may not do what was asked. Adapters return it,
// or wrap it, so that no caller needs the SDK's error types to tell a
// credentials problem from an outage.
var ErrUnauthorized = errors.New("unauthorized")

// ErrTruncated reports that a listing was larger than an adapter is willing to
// read, so any answer built from it would be incomplete. It is an error, not a
// flag, so that a partial listing can never be mistaken for a whole one.
var ErrTruncated = errors.New("listing too large to read in full")

// ErrUnsupported reports that a request makes no sense for the kind of
// resource it names, such as asking for the incidents of an SSL check. Like
// [ErrNotFound] it lets a handler explain the problem without knowing which
// API call would have failed.
var ErrUnsupported = errors.New("unsupported")

// CheckType identifies the kind of check.
type CheckType string

const (
	CheckTypeUptime    CheckType = "uptime"
	CheckTypePagespeed CheckType = "pagespeed"
	CheckTypeSSL       CheckType = "ssl"
	CheckTypeHeartbeat CheckType = "heartbeat"
)

// String implements [fmt.Stringer].
func (t CheckType) String() string { return string(t) }

var checkTypes = []CheckType{
	CheckTypeUptime,
	CheckTypePagespeed,
	CheckTypeSSL,
	CheckTypeHeartbeat,
}

// ParseCheckType validates a raw string, such as one supplied by a tool caller.
// The error names the valid values so that a model can correct itself.
func ParseCheckType(raw string) (CheckType, error) {
	t := CheckType(strings.ToLower(strings.TrimSpace(raw)))
	if !slices.Contains(checkTypes, t) {
		return "", fmt.Errorf("invalid check type %q: must be one of %s", raw, oneOf(checkTypes))
	}
	return t, nil
}

// CheckStatus is the state of a check as an operator would describe it.
type CheckStatus string

const (
	CheckStatusUp     CheckStatus = "up"
	CheckStatusDown   CheckStatus = "down"
	CheckStatusPaused CheckStatus = "paused"

	// CheckStatusUnknown is reported for checks whose API does not expose a
	// health state, such as SSL and page speed checks that are not paused. It
	// is output only: [ParseCheckStatus] does not accept it, because there is
	// nothing useful to filter on.
	CheckStatusUnknown CheckStatus = "unknown"
)

var checkStatuses = []CheckStatus{
	CheckStatusUp,
	CheckStatusDown,
	CheckStatusPaused,
}

// ParseCheckStatus validates a raw string. See [ParseCheckType].
func ParseCheckStatus(raw string) (CheckStatus, error) {
	s := CheckStatus(strings.ToLower(strings.TrimSpace(raw)))
	if !slices.Contains(checkStatuses, s) {
		return "", fmt.Errorf("invalid check status %q: must be one of %s", raw, oneOf(checkStatuses))
	}
	return s, nil
}

// String implements [fmt.Stringer].
func (s CheckStatus) String() string { return string(s) }

// triageRank orders statuses by how much attention an operator should give
// them: broken first, then deliberately silenced, then healthy.
func (s CheckStatus) triageRank() int {
	switch s {
	case CheckStatusDown:
		return 0
	case CheckStatusPaused:
		return 1
	default:
		return 2
	}
}

// CheckRef identifies a check. A check is identified by its type as well as its
// id, because each type is a separate resource in the StatusCake API and ids
// are not guaranteed to be unique across them.
type CheckRef struct {
	Type CheckType
	ID   string
}

// CheckSummary is the cheap-to-list view of a check: enough to answer "what do
// we have?" and "what is down?", and to find the [CheckRef] for a closer look.
type CheckSummary struct {
	CheckRef

	Name          string
	Status        CheckStatus
	Tags          []string
	ContactGroups []string

	// Uptime is the percentage of time, from 0 to 100, that the check has been
	// up. It is absent for checks that have no such measure (page speed and SSL
	// checks), and for listings that were not asked to include it: the API has to
	// work it out for every check, which is expensive, so it is opt-in. See
	// [CheckFilter.IncludeUptime].
	Uptime gofp.Option[float64]
}

// Check is a summary plus the settings specific to its type.
type Check struct {
	CheckSummary

	Settings CheckSettings
}

// NewCheck combines a summary with settings. The check's type is taken from the
// settings, so the two can never disagree.
func NewCheck(summary CheckSummary, settings CheckSettings) Check {
	summary.Type = settings.Type()

	return Check{
		CheckSummary: summary,
		Settings:     settings,
	}
}

// CheckSettings is the configuration specific to one type of check.
//
// It is a sum type: a value is exactly one of [UptimeSettings],
// [PageSpeedSettings], [SSLSettings] or [HeartbeatSettings]. The unexported
// method seals the interface, so no other package can add a variant. Use
// [FoldSettings] to handle every variant; the compiler then rejects any call
// site that has not been updated when a variant is added.
//
// The fields of the variants below are illustrative. Keep the ones an operator
// would want to see when asking "why is this check behaving like this?", and
// leave out anything that is secret or high-volume.
type CheckSettings interface {
	// Type reports which kind of check these settings configure.
	Type() CheckType

	sealed()
}

// UptimeSettings configures an uptime check.
type UptimeSettings struct {
	Target   string
	Protocol string
	Interval time.Duration
}

// PageSpeedSettings configures a page speed check.
type PageSpeedSettings struct {
	Target   string
	Region   string
	Interval time.Duration
}

// SSLSettings configures an SSL certificate check.
type SSLSettings struct {
	Target    string
	Interval  time.Duration
	AlertDays []int
}

// HeartbeatSettings configures a heartbeat (push) check. The URL that a
// monitored job pings is deliberately not modelled: if it embeds a token, it
// must not reach a model's context.
type HeartbeatSettings struct {
	Period time.Duration
}

func (UptimeSettings) Type() CheckType    { return CheckTypeUptime }
func (PageSpeedSettings) Type() CheckType { return CheckTypePagespeed }
func (SSLSettings) Type() CheckType       { return CheckTypeSSL }
func (HeartbeatSettings) Type() CheckType { return CheckTypeHeartbeat }

func (UptimeSettings) sealed()    {}
func (PageSpeedSettings) sealed() {}
func (SSLSettings) sealed()       {}
func (HeartbeatSettings) sealed() {}

// FoldSettings reduces [CheckSettings] to a single value by applying the
// function that matches its variant. It plays the same role for this sum type
// as [gofp.OptionFold] does for Option: the caller must supply a function for
// every case, so adding a variant to this signature breaks every caller until
// it is handled.
func FoldSettings[R any](s CheckSettings, uptime func(UptimeSettings) R, pageSpeed func(PageSpeedSettings) R, ssl func(SSLSettings) R, heartbeat func(HeartbeatSettings) R) R {
	switch v := s.(type) {
	case UptimeSettings:
		return uptime(v)
	case PageSpeedSettings:
		return pageSpeed(v)
	case SSLSettings:
		return ssl(v)
	case HeartbeatSettings:
		return heartbeat(v)
	default:
		panic(fmt.Sprintf("domain: unhandled check settings %T", s))
	}
}

// CheckFilter narrows a listing. An absent filter matches everything.
type CheckFilter struct {
	Type   gofp.Option[CheckType]
	Status gofp.Option[CheckStatus]
	Tag    gofp.Option[string]

	// IncludeUptime asks for [CheckSummary.Uptime] to be filled in. Adapters
	// must not request it from the API otherwise.
	IncludeUptime bool

	// Limit is the maximum number of summaries wanted. Adapters should pass the
	// complete set of matches to [NewCheckList], which applies it.
	Limit int
}

// CheckList is one page of check summaries.
type CheckList struct {
	// Checks holds at most the requested limit, ordered for triage.
	Checks []CheckSummary

	// Total is the number of checks that matched the filter, before the limit.
	Total int
}

// Matches reports whether a summary satisfies every filter that is present.
// The limit is not a criterion and is applied later, by [NewCheckList].
func (f CheckFilter) Matches(s CheckSummary) bool {
	if t, ok := f.Type.TryUnwrap(); ok && s.Type != t {
		return false
	}
	if st, ok := f.Status.TryUnwrap(); ok && s.Status != st {
		return false
	}
	if tag, ok := f.Tag.TryUnwrap(); ok && !slices.Contains(s.Tags, tag) {
		return false
	}
	return true
}

// NewCheckList orders every match for triage (down first, then paused, then up;
// alphabetical within each group) and keeps the first limit of them, so the
// checks an operator most needs to see survive truncation. A limit of zero or
// less keeps everything.
//
// Putting this policy here, rather than in each adapter, keeps it in one place.
func NewCheckList(matches []CheckSummary, limit int) CheckList {
	sorted := slices.Clone(matches)
	slices.SortStableFunc(sorted, func(a, b CheckSummary) int {
		if c := cmp.Compare(a.Status.triageRank(), b.Status.triageRank()); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	total := len(sorted)
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return CheckList{Checks: sorted, Total: total}
}

// Truncated reports whether more checks matched than the list holds.
func (l CheckList) Truncated() bool {
	return l.Total > len(l.Checks)
}

func oneOf[T ~string](values []T) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = string(v)
	}
	return strings.Join(parts, ", ")
}
