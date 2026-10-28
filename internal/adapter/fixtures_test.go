package adapter_test

// Fixtures are shaped like the API's JSON rather than like the SDK's structs,
// so that decoding the wire format is part of what the tests cover. Each one
// carries personal or secret data (addresses, numbers, tokens in URLs) that the
// adapter must not pass on.

func contactGroup(id, name string, emails, mobiles, integrations int) map[string]any {
	pad := func(n int, v string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = v
		}
		return out
	}

	return map[string]any{
		"id":              id,
		"name":            name,
		"email_addresses": pad(emails, "oncall@example.com"),
		"mobile_numbers":  pad(mobiles, "+447700900123"),
		"integrations":    pad(integrations, "99"),
		"ping_url":        "https://hooks.example.com/SECRET",
	}
}

func heartbeatOverview(id, name, status string, paused bool, tags ...string) map[string]any {
	if tags == nil {
		tags = []string{}
	}

	return map[string]any{
		"id":     id,
		"name":   name,
		"url":    "https://heartbeat.statuscake.com/SECRET-TOKEN",
		"period": 90000,
		"paused": paused,
		"status": status,
		"tags":   tags,
	}
}

func maintenanceWindow(id, name, state, repeat string, tags, tests []string) map[string]any {
	return map[string]any{
		"id":              id,
		"name":            name,
		"state":           state,
		"repeat_interval": repeat,
		"start_at":        "2026-10-20T01:00:00+00:00",
		"end_at":          "2026-10-20T03:00:00+00:00",
		"timezone":        "Europe/London",
		"tags":            tags,
		"tests":           tests,
	}
}

func pagespeed(id, name string, paused bool) map[string]any {
	return map[string]any{
		"id":            id,
		"name":          name,
		"website_url":   "https://" + name + ".example.com",
		"check_rate":    3600,
		"alert_bigger":  0,
		"alert_slower":  0,
		"alert_smaller": 0,
		"location":      "UK",
		"paused":        paused,
	}
}

func ssl(id, host string, paused bool) map[string]any {
	return map[string]any{
		"id":            id,
		"website_url":   "https://" + host,
		"check_rate":    86400,
		"alert_at":      []int{30, 7, 1},
		"mixed_content": []any{},
		"paused":        paused,
	}
}

func uptimeDetail(id string) map[string]any {
	return map[string]any{
		"id":             id,
		"name":           "Checkout API",
		"test_type":      "HTTP",
		"website_url":    "https://api.example.com/health?token=SECRET",
		"check_rate":     60,
		"confirmation":   2,
		"contact_groups": []string{"1"},
		"dns_ips":        []string{},
		"paused":         false,
		"servers":        []any{},
		"status":         "down",
		"status_codes":   []string{},
		"tags":           []string{"production", "payments"},
		"timeout":        15,
		"trigger_rate":   0,
		"uptime":         99.5,
	}
}

func uptimeOverview(id, name, status string, paused bool, tags ...string) map[string]any {
	if tags == nil {
		tags = []string{}
	}

	return map[string]any{
		"id":          id,
		"name":        name,
		"website_url": "https://" + name + ".example.com",
		"test_type":   "HTTP",
		"check_rate":  300,
		"paused":      paused,
		"status":      status,
		"tags":        tags,
	}
}
