package tools

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

const (
	defaultLimit = 25
	maxLimit     = 100
)

// clampLimit applies the page size policy for listing tools. Anything unset or
// invalid gets the default; anything too large is capped. A hard cap matters
// because every returned item costs the model context.
func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	default:
		return n
	}
}

// optional turns a tool argument that may be empty into an [gofp.Option],
// parsing it when present. Tool inputs are plain strings, because the SDK
// infers their JSON schema by reflection and cannot describe an Option; the
// Option appears one step further in, in the domain.
func optional[T any](raw string, parse func(string) (T, error)) (gofp.Option[T], error) {
	if strings.TrimSpace(raw) == "" {
		return gofp.None[T](), nil
	}
	v, err := parse(raw)
	if err != nil {
		return gofp.None[T](), err
	}
	return gofp.Some(v), nil
}

// optionalString is [optional] for arguments that need no parsing.
func optionalString(raw string) gofp.Option[string] {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return gofp.None[string]()
	}
	return gofp.Some(raw)
}

// parseTime reads an RFC 3339 time. The error shows an example, since a model
// that is told what it got wrong but not what is wanted tends to guess again.
func parseTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: use RFC 3339, such as 2026-10-01T09:30:00Z", raw)
	}
	return t, nil
}

// optionalWindow parses the before and after arguments shared by the tools
// that look back in time, and rejects a window that cannot contain anything.
func optionalWindow(before, after string) (gofp.Option[time.Time], gofp.Option[time.Time], error) {
	none := gofp.None[time.Time]()

	b, err := optional(before, parseTime)
	if err != nil {
		return none, none, fmt.Errorf("before: %w", err)
	}
	a, err := optional(after, parseTime)
	if err != nil {
		return none, none, fmt.Errorf("after: %w", err)
	}

	bt, hasBefore := b.TryUnwrap()
	at, hasAfter := a.TryUnwrap()
	if hasBefore && hasAfter && !at.Before(bt) {
		return none, none, errors.New("after must be earlier than before")
	}
	return b, a, nil
}

// parseRef reads the type and id arguments that identify one check.
func parseRef(rawType, rawID string) (domain.CheckRef, error) {
	typ, err := domain.ParseCheckType(rawType)
	if err != nil {
		return domain.CheckRef{}, err
	}

	id := strings.TrimSpace(rawID)
	if id == "" {
		return domain.CheckRef{}, errors.New("id is required; use list_checks to find it")
	}
	return domain.CheckRef{Type: typ, ID: id}, nil
}
