package tools

import (
	"time"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

// The types in this file are what the model reads. They are deliberately plain:
// strings, numbers and slices, snake_case names, RFC 3339 times, and durations
// in an explicit unit. They are a presentation of the domain, not the domain
// itself, so they can change shape to suit the model without touching it.

// CheckSummaryView is one row in a listing, and the header of a single check.
type CheckSummaryView struct {
	ID           string   `json:"id" jsonschema:"identifier, unique within its type; pass it to get_check together with type"`
	Type         string   `json:"type" jsonschema:"one of: uptime, pagespeed, ssl, heartbeat"`
	Name         string   `json:"name"`
	Status       string   `json:"status" jsonschema:"one of: up, down, paused"`
	Tags         []string `json:"tags"`
	LastTestedAt string   `json:"last_tested_at,omitempty" jsonschema:"RFC 3339 time of the most recent test; absent if the check has never run"`
}

func toSummaryView(c domain.CheckSummary) CheckSummaryView {
	tags := c.Tags
	if tags == nil {
		tags = []string{} // marshal as [] rather than null
	}

	return CheckSummaryView{
		ID:     c.ID,
		Type:   string(c.Type),
		Name:   c.Name,
		Status: string(c.Status),
		Tags:   tags,
	}
}

// CheckHistoryEntryView is the outcome of one test of a check.
type CheckHistoryEntryView struct {
	At             string `json:"at" jsonschema:"RFC 3339 time of the test"`
	Location       string `json:"location,omitempty" jsonschema:"where the test ran from"`
	StatusCode     int    `json:"status_code,omitempty" jsonschema:"HTTP status the target answered with; absent if it did not answer"`
	ResponseTimeMs int64  `json:"response_time_ms,omitempty" jsonschema:"how long the target took to answer; absent if it did not answer"`
}

func toHistoryEntryView(e domain.CheckHistoryEntry) CheckHistoryEntryView {
	return CheckHistoryEntryView{
		At:             formatTime(e.At),
		Location:       e.Location,
		StatusCode:     e.StatusCode.UnwrapOr(0),
		ResponseTimeMs: e.ResponseTime.UnwrapOr(0).Milliseconds(),
	}
}

// IncidentView is a period during which a check was down.
type IncidentView struct {
	StartedAt       string `json:"started_at" jsonschema:"RFC 3339 time the check went down"`
	EndedAt         string `json:"ended_at,omitempty" jsonschema:"RFC 3339 time the check recovered; absent while the incident is ongoing"`
	Ongoing         bool   `json:"ongoing" jsonschema:"true if the check is still down"`
	DurationSeconds int64  `json:"duration_seconds" jsonschema:"how long the check was down, or has been down so far"`
}

func toIncidentView(i domain.Incident, now time.Time) IncidentView {
	return IncidentView{
		StartedAt: formatTime(i.Started),
		EndedAt: gofp.OptionFold(i.Ended,
			func() string { return "" },
			formatTime,
		),
		Ongoing:         i.Ongoing(),
		DurationSeconds: int64(i.Duration(now) / time.Second),
	}
}

// ContactGroupView is a group of recipients. Only the number of each kind is
// shown: the addresses and numbers themselves are personal data.
type ContactGroupView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Emails       int    `json:"emails" jsonschema:"number of email addresses in the group"`
	Mobiles      int    `json:"mobiles" jsonschema:"number of mobile numbers in the group"`
	Integrations int    `json:"integrations" jsonschema:"number of integrations, such as chat or paging services, in the group"`
}

func toContactGroupView(g domain.ContactGroup) ContactGroupView {
	return ContactGroupView{
		ID:           g.ID,
		Name:         g.Name,
		Emails:       g.Emails,
		Mobiles:      g.Mobiles,
		Integrations: g.Integrations,
	}
}

// MaintenanceWindowView is a period during which alerts are suppressed.
type MaintenanceWindowView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	State    string   `json:"state" jsonschema:"one of: active, pending, paused"`
	Start    string   `json:"start" jsonschema:"RFC 3339 time the window opens"`
	End      string   `json:"end" jsonschema:"RFC 3339 time the window closes"`
	Repeat   string   `json:"repeat" jsonschema:"how often the window recurs, such as never or weekly"`
	Timezone string   `json:"timezone"`
	Tags     []string `json:"tags" jsonschema:"checks carrying any of these tags are covered"`
	Checks   int      `json:"checks" jsonschema:"number of checks covered by name, in addition to those covered by tag"`
}

func toMaintenanceWindowView(w domain.MaintenanceWindow) MaintenanceWindowView {
	tags := w.Tags
	if tags == nil {
		tags = []string{}
	}

	return MaintenanceWindowView{
		ID:       w.ID,
		Name:     w.Name,
		State:    w.State.String(),
		Start:    formatTime(w.Start),
		End:      formatTime(w.End),
		Repeat:   w.Repeat,
		Timezone: w.Timezone,
		Tags:     tags,
		Checks:   w.Checks,
	}
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// SettingsView is the model-facing form of a check's type-specific settings. It
// is an interface so that the schema the SDK infers for a field of this type is
// unrestricted; the shape that actually appears is determined by the sibling
// "type" field.
type SettingsView interface {
	settingsView()
}

// UptimeSettingsView is the settings of an uptime check.
type UptimeSettingsView struct {
	Target          string `json:"target" jsonschema:"URL or host being monitored"`
	Protocol        string `json:"protocol"`
	IntervalSeconds int    `json:"interval_seconds" jsonschema:"seconds between tests"`
}

// PageSpeedSettingsView is the settings of a page speed check.
type PageSpeedSettingsView struct {
	Target          string `json:"target" jsonschema:"URL being monitored"`
	Region          string `json:"region" jsonschema:"region the page is tested from"`
	IntervalSeconds int    `json:"interval_seconds" jsonschema:"seconds between tests"`
}

// SSLSettingsView is the settings of an SSL certificate check.
type SSLSettingsView struct {
	Target          string `json:"target" jsonschema:"host whose certificate is monitored"`
	IntervalSeconds int    `json:"interval_seconds" jsonschema:"seconds between tests"`
	AlertDays       []int  `json:"alert_days" jsonschema:"days before expiry at which alerts are sent"`
}

// HeartbeatSettingsView is the settings of a heartbeat check.
type HeartbeatSettingsView struct {
	PeriodSeconds int `json:"period_seconds" jsonschema:"longest silence, in seconds, before the check is considered down"`
}

func (UptimeSettingsView) settingsView()    {}
func (PageSpeedSettingsView) settingsView() {}
func (SSLSettingsView) settingsView()       {}
func (HeartbeatSettingsView) settingsView() {}

// toSettingsView folds the domain's sum type into the matching view. Because it
// uses [domain.FoldSettings], this function stops compiling the moment a new
// variant is added, which is the point.
func toSettingsView(s domain.CheckSettings) SettingsView {
	return domain.FoldSettings(s,
		func(v domain.UptimeSettings) SettingsView {
			return UptimeSettingsView{Target: v.Target, Protocol: v.Protocol, IntervalSeconds: seconds(v.Interval)}
		},
		func(v domain.PageSpeedSettings) SettingsView {
			return PageSpeedSettingsView{Target: v.Target, Region: v.Region, IntervalSeconds: seconds(v.Interval)}
		},
		func(v domain.SSLSettings) SettingsView {
			days := v.AlertDays
			if days == nil {
				days = []int{}
			}
			return SSLSettingsView{Target: v.Target, IntervalSeconds: seconds(v.Interval), AlertDays: days}
		},
		func(v domain.HeartbeatSettings) SettingsView {
			return HeartbeatSettingsView{PeriodSeconds: seconds(v.Period)}
		},
	)
}

func seconds(d time.Duration) int {
	return int(d / time.Second)
}
