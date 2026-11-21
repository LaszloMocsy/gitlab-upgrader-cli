package config

import (
	"testing"
	"time"
)

func mustWindow(t *testing.T, spec string) MaintenanceWindow {
	t.Helper()
	var w MaintenanceWindow
	if err := w.UnmarshalText([]byte(spec)); err != nil {
		t.Fatalf("%q: %v", spec, err)
	}
	return w
}

func TestWindowParse(t *testing.T) {
	for _, spec := range []string{
		"Sat 02:00-06:00 UTC",
		"Sat 02:00-06:00",
		"sat,SUN 02:00-06:00",
		"Mon-Fri 22:00-02:00 Europe/Budapest",
		"Fri-Mon 00:00-24:00",
		"daily 01:00-05:00 America/New_York",
	} {
		mustWindow(t, spec)
	}
	for _, spec := range []string{
		"Sat",
		"02:00-06:00",
		"Caturday 02:00-06:00",
		"Sat 2:00-06:00",
		"Sat 02:00-06:60",
		"Sat 24:00-06:00",
		"Sat 02:00-24:01",
		"Sat 02:00-02:00",
		"Sat 02:00",
		"Sat 02:00-06:00 Mars/Olympus",
		"Sat 02:00-06:00 UTC extra",
		"Mon-Funday 02:00-06:00",
	} {
		var w MaintenanceWindow
		if err := w.UnmarshalText([]byte(spec)); err == nil {
			t.Errorf("%q: want an error", spec)
		}
	}
}

func TestWindowEmpty(t *testing.T) {
	w := mustWindow(t, "  ")
	if !w.IsZero() || !w.Contains(time.Now()) {
		t.Error("an empty window must allow any time")
	}
}

func TestWindowNormalizesSpacing(t *testing.T) {
	if got := mustWindow(t, "  Sat   02:00-06:00  UTC ").String(); got != "Sat 02:00-06:00 UTC" {
		t.Errorf("got %q", got)
	}
}

func TestWindowContains(t *testing.T) {
	utc := func(s string) time.Time {
		tm, err := time.Parse("2006-01-02 15:04 MST", s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	// 2026-11-21 is a Saturday.
	tests := []struct {
		spec string
		at   string
		want bool
	}{
		{"Sat 02:00-06:00 UTC", "2026-11-21 02:00 UTC", true},
		{"Sat 02:00-06:00 UTC", "2026-11-21 05:59 UTC", true},
		{"Sat 02:00-06:00 UTC", "2026-11-21 06:00 UTC", false},
		{"Sat 02:00-06:00 UTC", "2026-11-21 01:59 UTC", false},
		{"Sat 02:00-06:00 UTC", "2026-11-22 03:00 UTC", false},
		// Crossing midnight: Friday 22:00 until Saturday 02:00.
		{"Fri 22:00-02:00", "2026-11-21 01:30 UTC", true},
		{"Fri 22:00-02:00", "2026-11-20 23:00 UTC", true},
		{"Fri 22:00-02:00", "2026-11-21 23:00 UTC", false},
		{"Fri 22:00-02:00", "2026-11-20 21:00 UTC", false},
		// Day ranges wrap around the week.
		{"Fri-Mon 00:00-24:00", "2026-11-23 12:00 UTC", true},
		{"Fri-Mon 00:00-24:00", "2026-11-24 12:00 UTC", false},
		// Zones: 02:00 in Budapest (UTC+1 in November) is 01:00 UTC.
		{"Sat 02:00-03:00 Europe/Budapest", "2026-11-21 01:30 UTC", true},
		{"Sat 02:00-03:00 Europe/Budapest", "2026-11-21 02:30 UTC", false},
		{"daily 01:00-05:00", "2026-11-25 04:00 UTC", true},
	}
	for _, tt := range tests {
		if got := mustWindow(t, tt.spec).Contains(utc(tt.at)); got != tt.want {
			t.Errorf("%q at %s: got %v, want %v", tt.spec, tt.at, got, tt.want)
		}
	}
}
