package adapters

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/tomasbasham/gofp"

	"github.com/StatusCakeDev/statuscake-go"

	"github.com/StatusCakeDev/statuscake-mcp/internal/domain"
)

// pageSize is how many items to ask the API for at a time, and maxPages bounds
// how many pages are read for one listing, so that an unexpectedly large
// account cannot turn one tool call into an unbounded number of requests.
const (
	pageSize = 100
	maxPages = 50
)

// Client implements the ports that the tools package consumes, in terms of the
// StatusCake SDK. It is the only place that knows the SDK's types.
type Client struct {
	logger *slog.Logger
	client *statuscake.Client
}

func NewClient(logger *slog.Logger, client *statuscake.Client) *Client {
	return &Client{
		logger: logger,
		client: client,
	}
}

func (c *Client) GetCheck(ctx context.Context, ref domain.CheckRef) (domain.Check, error) {
	var (
		check domain.Check
		err   error
	)

	switch ref.Type {
	case domain.CheckTypeUptime:
		check, err = c.getUptimeCheck(ctx, ref.ID)
	case domain.CheckTypeHeartbeat:
		check, err = c.getHeartbeatCheck(ctx, ref.ID)
	case domain.CheckTypePagespeed:
		check, err = c.getPagespeedCheck(ctx, ref.ID)
	case domain.CheckTypeSSL:
		check, err = c.getSSLCheck(ctx, ref.ID)
	default:
		return domain.Check{}, fmt.Errorf("%w: check type %q", domain.ErrUnsupported, ref.Type)
	}

	return check, translate(fmt.Sprintf("get %s check", ref.Type), err)
}

// ListChecks gathers every check of the requested type, or of every type when
// none is requested, and applies the filter here rather than in the API. The
// API can only filter some types, and not all by the same criteria, so
// filtering once, over the domain's summaries, gives every type identical
// behaviour and lets [domain.NewCheckList] report an honest total.
func (c *Client) ListChecks(ctx context.Context, filter domain.CheckFilter) (domain.CheckList, error) {
	sources := []struct {
		checkType domain.CheckType
		listFn    func(context.Context) ([]domain.CheckSummary, error)
	}{
		{domain.CheckTypeUptime, c.listUptimeChecks},
		{domain.CheckTypeHeartbeat, c.listHeartbeatChecks},
		{domain.CheckTypePagespeed, c.listPagespeedChecks},
		{domain.CheckTypeSSL, c.listSSLChecks},
	}

	var matches []domain.CheckSummary
	for _, src := range sources {
		// Skip the request altogether when the type cannot match.
		if t, ok := filter.Type.TryUnwrap(); ok && t != src.checkType {
			continue
		}

		summaries, err := src.listFn(ctx)
		if err != nil {
			return domain.CheckList{}, translate(fmt.Sprintf("list %s checks", src.checkType), err)
		}

		for _, s := range summaries {
			if filter.Matches(s) {
				matches = append(matches, s)
			}
		}
	}

	return domain.NewCheckList(matches, filter.Limit), nil
}

// ListCheckHistory lists the tests an uptime check has run. Other types of
// check have no history that this server exposes.
func (c *Client) ListCheckHistory(ctx context.Context, filter domain.CheckHistoryFilter) (domain.CheckHistory, error) {
	if filter.Type != domain.CheckTypeUptime {
		return domain.CheckHistory{}, fmt.Errorf("%w: history for %s checks", domain.ErrUnsupported, filter.Type)
	}

	req := c.client.ListUptimeTestHistory(ctx, filter.ID)
	req = req.Limit(int32(askFor(filter.Limit)))
	if t, ok := filter.Before.TryUnwrap(); ok {
		req = req.Before(t.Unix())
	}
	if t, ok := filter.After.TryUnwrap(); ok {
		req = req.After(t.Unix())
	}

	res, err := req.Execute()
	if err != nil {
		return domain.CheckHistory{}, translate("list check history", err)
	}

	entries := make([]domain.CheckHistoryEntry, len(res.Data))
	for i, r := range res.Data {
		entries[i] = domain.CheckHistoryEntry{
			At:           r.Created,
			Location:     deref(r.Location),
			StatusCode:   optionFrom(r.StatusCode, toInt),
			ResponseTime: optionFrom(r.Performance, toDuration),
		}
	}

	return domain.NewCheckHistory(entries, filter.Limit), nil
}

// ListIncidents lists the periods during which an uptime check was down.
//
// The API exposes periods of both kinds, up and down, so the down ones are
// picked out here. Only one page of periods is read: incidents are wanted
// newest first and in small numbers, and the model is told when there is more.
func (c *Client) ListIncidents(ctx context.Context, filter domain.IncidentFilter) (domain.IncidentList, error) {
	if filter.Type != domain.CheckTypeUptime {
		return domain.IncidentList{}, fmt.Errorf("%w: incidents for %s checks", domain.ErrUnsupported, filter.Type)
	}

	req := c.client.ListUptimeTestPeriods(ctx, filter.ID).Limit(pageSize)
	if t, ok := filter.Before.TryUnwrap(); ok {
		req = req.Before(t.Unix())
	}
	if t, ok := filter.After.TryUnwrap(); ok {
		req = req.After(t.Unix())
	}

	res, err := req.Execute()
	if err != nil {
		return domain.IncidentList{}, translate("list incidents", err)
	}

	var incidents []domain.Incident
	for _, p := range res.Data {
		if p.Status != statuscake.UptimeTestStatusDown {
			continue
		}
		incidents = append(incidents, domain.Incident{
			Started: p.Created,
			Ended:   optionFrom(p.Ended, toSelf),
		})
	}

	// A full page suggests the window holds more periods than were read.
	more := len(res.Data) >= pageSize
	return domain.NewIncidentList(incidents, filter.Limit, more), nil
}

func (c *Client) ListContactGroups(ctx context.Context, filter domain.ContactGroupFilter) (domain.ContactGroupList, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.ContactGroup, statuscake.Pagination, error) {
		res, err := c.client.ListContactGroups(ctx).Page(page).Limit(limit).Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return domain.ContactGroupList{}, translate("list contact groups", err)
	}

	groups := make([]domain.ContactGroup, len(items))
	for i, g := range items {
		groups[i] = domain.ContactGroup{
			ID:           g.ID,
			Name:         g.Name,
			Emails:       len(g.EmailAddresses),
			Mobiles:      len(g.MobileNumbers),
			Integrations: len(g.Integrations),
		}
	}

	return domain.NewContactGroupList(groups, filter.Limit), nil
}

func (c *Client) ListMaintenanceWindows(ctx context.Context, filter domain.MaintenanceWindowFilter) (domain.MaintenanceWindowList, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.MaintenanceWindow, statuscake.Pagination, error) {
		req := c.client.ListMaintenanceWindows(ctx).Page(page).Limit(limit)
		if s, ok := filter.State.TryUnwrap(); ok {
			req = req.State(string(s))
		}

		res, err := req.Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return domain.MaintenanceWindowList{}, translate("list maintenance windows", err)
	}

	windows := make([]domain.MaintenanceWindow, len(items))
	for i, w := range items {
		windows[i] = domain.MaintenanceWindow{
			ID:       w.ID,
			Name:     w.Name,
			State:    domain.MaintenanceState(w.State),
			Start:    w.Start,
			End:      w.End,
			Repeat:   repeatName(w.RepeatInterval),
			Timezone: w.Timezone,
			Tags:     w.Tags,
			Checks:   len(w.Tests),
		}
	}

	return domain.NewMaintenanceWindowList(windows, filter.Limit), nil
}

func (c *Client) listUptimeChecks(ctx context.Context) ([]domain.CheckSummary, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.UptimeTestOverview, statuscake.Pagination, error) {
		// The uptime percentage is not used, and costs the API work to compute.
		res, err := c.client.ListUptimeTests(ctx).Page(page).Limit(limit).Nouptime(true).Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return nil, err
	}

	checks := make([]domain.CheckSummary, len(items))
	for i, t := range items {
		checks[i] = domain.CheckSummary{
			CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: t.ID},
			Name:     t.Name,
			Status:   healthStatus(t.Paused, string(t.Status)),
			Tags:     t.Tags,
		}
	}
	return checks, nil
}

func (c *Client) getUptimeCheck(ctx context.Context, id string) (domain.Check, error) {
	res, err := c.client.GetUptimeTest(ctx, id).Execute()
	if err != nil {
		return domain.Check{}, err
	}

	t := res.Data
	return domain.NewCheck(
		domain.CheckSummary{
			CheckRef: domain.CheckRef{Type: domain.CheckTypeUptime, ID: t.ID},
			Name:     t.Name,
			Status:   healthStatus(t.Paused, string(t.Status)),
			Tags:     t.Tags,
		},
		domain.UptimeSettings{
			Target:   t.WebsiteURL,
			Protocol: string(t.TestType),
			Interval: time.Duration(t.CheckRate) * time.Second,
		},
	), nil
}

func (c *Client) listHeartbeatChecks(ctx context.Context) ([]domain.CheckSummary, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.HeartbeatTestOverview, statuscake.Pagination, error) {
		res, err := c.client.ListHeartbeatTests(ctx).Page(page).Limit(limit).Nouptime(true).Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return nil, err
	}

	checks := make([]domain.CheckSummary, len(items))
	for i, t := range items {
		checks[i] = domain.CheckSummary{
			CheckRef: domain.CheckRef{Type: domain.CheckTypeHeartbeat, ID: t.ID},
			Name:     t.Name,
			Status:   healthStatus(t.Paused, string(t.Status)),
			Tags:     t.Tags,
		}
	}
	return checks, nil
}

func (c *Client) getHeartbeatCheck(ctx context.Context, id string) (domain.Check, error) {
	res, err := c.client.GetHeartbeatTest(ctx, id).Execute()
	if err != nil {
		return domain.Check{}, err
	}

	t := res.Data
	return domain.NewCheck(
		domain.CheckSummary{
			CheckRef: domain.CheckRef{Type: domain.CheckTypeHeartbeat, ID: t.ID},
			Name:     t.Name,
			Status:   healthStatus(t.Paused, string(t.Status)),
			Tags:     t.Tags,
		},
		// The URL a job pings is deliberately left out. See HeartbeatSettings.
		domain.HeartbeatSettings{Period: time.Duration(t.Period) * time.Second},
	), nil
}

func (c *Client) listPagespeedChecks(ctx context.Context) ([]domain.CheckSummary, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.PagespeedTest, statuscake.Pagination, error) {
		res, err := c.client.ListPagespeedTests(ctx).Page(page).Limit(limit).Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return nil, err
	}

	checks := make([]domain.CheckSummary, len(items))
	for i, t := range items {
		checks[i] = pagespeedSummary(t)
	}
	return checks, nil
}

func (c *Client) getPagespeedCheck(ctx context.Context, id string) (domain.Check, error) {
	res, err := c.client.GetPagespeedTest(ctx, id).Execute()
	if err != nil {
		return domain.Check{}, err
	}

	t := res.Data
	return domain.NewCheck(pagespeedSummary(t), domain.PageSpeedSettings{
		Target:   t.WebsiteURL,
		Region:   t.Location,
		Interval: time.Duration(t.CheckRate) * time.Second,
	}), nil
}

func (c *Client) listSSLChecks(ctx context.Context) ([]domain.CheckSummary, error) {
	items, err := collect(func(page, limit int32) ([]statuscake.SSLTest, statuscake.Pagination, error) {
		res, err := c.client.ListSslTests(ctx).Page(page).Limit(limit).Execute()
		return res.Data, res.Metadata, err
	})
	if err != nil {
		return nil, err
	}

	checks := make([]domain.CheckSummary, len(items))
	for i, t := range items {
		checks[i] = sslSummary(t)
	}
	return checks, nil
}

func (c *Client) getSSLCheck(ctx context.Context, id string) (domain.Check, error) {
	res, err := c.client.GetSslTest(ctx, id).Execute()
	if err != nil {
		return domain.Check{}, err
	}

	t := res.Data
	days := make([]int, len(t.AlertAt))
	for i, d := range t.AlertAt {
		days[i] = int(d)
	}

	return domain.NewCheck(sslSummary(t), domain.SSLSettings{
		Target:    t.WebsiteURL,
		Interval:  time.Duration(t.CheckRate) * time.Second,
		AlertDays: days,
	}), nil
}

// pagespeedSummary and sslSummary exist because the list and get endpoints
// return the same model, and a check must look the same however it was found.
// Neither type has tags, and SSL checks have no name, so the monitored address
// stands in for it.
func pagespeedSummary(t statuscake.PagespeedTest) domain.CheckSummary {
	return domain.CheckSummary{
		CheckRef: domain.CheckRef{Type: domain.CheckTypePagespeed, ID: t.ID},
		Name:     t.Name,
		Status:   unobservedStatus(t.Paused),
	}
}

func sslSummary(t statuscake.SSLTest) domain.CheckSummary {
	return domain.CheckSummary{
		CheckRef: domain.CheckRef{Type: domain.CheckTypeSSL, ID: t.ID},
		Name:     t.WebsiteURL,
		Status:   unobservedStatus(t.Paused),
	}
}

// healthStatus derives the status an operator would give a check from the two
// facts the API reports separately. A paused check is not being tested, so
// whatever health it last reported is stale.
func healthStatus(paused bool, health string) domain.CheckStatus {
	switch {
	case paused:
		return domain.CheckStatusPaused
	case health == string(statuscake.UptimeTestStatusUp):
		return domain.CheckStatusUp
	case health == string(statuscake.UptimeTestStatusDown):
		return domain.CheckStatusDown
	default:
		return domain.CheckStatusUnknown
	}
}

// unobservedStatus is for check types whose API reports no health at all.
func unobservedStatus(paused bool) domain.CheckStatus {
	if paused {
		return domain.CheckStatusPaused
	}
	return domain.CheckStatusUnknown
}

func repeatName(r statuscake.MaintenanceWindowRepeatInterval) string {
	switch r {
	case statuscake.MaintenanceWindowRepeatIntervalNever:
		return "never"
	case statuscake.MaintenanceWindowRepeatIntervalDaily:
		return "daily"
	case statuscake.MaintenanceWindowRepeatIntervalWeekly:
		return "weekly"
	case statuscake.MaintenanceWindowRepeatIntervalBiweekly:
		return "fortnightly"
	case statuscake.MaintenanceWindowRepeatIntervalMonthly:
		return "monthly"
	default:
		return string(r)
	}
}

// askFor is how many items to request when the caller wants limit: one more, so
// that the domain can tell whether anything lies beyond.
func askFor(limit int) int {
	if limit <= 0 || limit >= pageSize {
		return pageSize
	}
	return limit + 1
}

// collect reads every page of a listing, up to maxPages.
func collect[T any](fetch func(page, limit int32) ([]T, statuscake.Pagination, error)) ([]T, error) {
	var all []T
	for page := int32(1); page <= maxPages; page++ {
		items, meta, err := fetch(page, pageSize)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if page >= meta.PageCount {
			break
		}
	}
	return all, nil
}

// translate turns an SDK error into the vocabulary of the domain, so that
// handlers can react to a missing resource without importing the SDK. Anything
// else is passed on with context, and keeps its cause for errors.As.
func translate(op string, err error) error {
	if err == nil {
		return nil
	}

	var apiErr statuscake.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return fmt.Errorf("%s: %w: %w", op, domain.ErrNotFound, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// This function is necessary because the SDK models use pointers to denote
// possibly null values. Thereofre for safety a null pointer check is handled in
// one place.
func deref[T any](val *T) T {
	if val == nil {
		var zero T
		return zero
	}
	return *val
}

// optionFrom lifts the SDK's nil-means-absent pointers into an Option.
func optionFrom[T, U any](p *T, convert func(T) U) gofp.Option[U] {
	if p == nil {
		return gofp.None[U]()
	}
	return gofp.Some(convert(*p))
}

// convenience functions to translate common values from the API into domain
// compatible types.

func toSelf[T any](val T) T {
	return val
}

func toInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](val T) int {
	return int(val)
}

func toDuration(ms int64) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
