package config

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	good := map[string]time.Duration{
		"0":     0,
		"0s":    0,
		"90m":   90 * time.Minute,
		"6h":    6 * time.Hour,
		"7d":    7 * 24 * time.Hour,
		"1d12h": 36 * time.Hour,
		"1.5h":  90 * time.Minute,
	}
	for in, want := range good {
		var d Duration
		if err := d.UnmarshalText([]byte(in)); err != nil || time.Duration(d) != want {
			t.Errorf("%q: got %v, %v", in, time.Duration(d), err)
		}
	}
	for _, in := range []string{"", "d", "7", "1w", "-1h", "1d-2h", "h1", "99999999999999d"} {
		var d Duration
		if err := d.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error, got %v", in, time.Duration(d))
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                       "0",
		6 * time.Hour:           "6h",
		90 * time.Minute:        "1h30m",
		48 * time.Hour:          "2d",
		36 * time.Hour:          "36h",
		45 * time.Second:        "45s",
		1500 * time.Millisecond: "1.5s",
	} {
		if got := formatDuration(d); got != want {
			t.Errorf("%v: got %q, want %q", d, got, want)
		}
	}
}

func TestDurationRoundTrip(t *testing.T) {
	for _, in := range []string{"0", "6h", "1h30m", "7d", "36h", "45s"} {
		var d Duration
		if err := d.UnmarshalText([]byte(in)); err != nil {
			t.Fatal(err)
		}
		out, _ := d.MarshalText()
		if string(out) != in {
			t.Errorf("%q round-trips to %q", in, out)
		}
	}
}

func TestSize(t *testing.T) {
	good := map[string]Size{
		"auto":    {Auto: true},
		"0":       {Bytes: 0},
		"1048576": {Bytes: 1 << 20},
		"50GiB":   {Bytes: 50 << 30},
		"50 gib":  {Bytes: 50 << 30},
		"500MB":   {Bytes: 500_000_000},
		"1.5TiB":  {Bytes: 3 << 39},
		"10B":     {Bytes: 10},
	}
	for in, want := range good {
		var z Size
		if err := z.UnmarshalText([]byte(in)); err != nil || z != want {
			t.Errorf("%q: got %+v, %v", in, z, err)
		}
	}
	for _, in := range []string{"", "lots", "-1GiB", "5 PB", "1e3", "GiB"} {
		var z Size
		if err := z.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error, got %+v", in, z)
		}
	}
	for z, want := range map[Size]string{
		{Auto: true}:         "auto",
		{Bytes: 50 << 30}:    "50GiB",
		{Bytes: 500_000_000}: "500MB",
		{Bytes: 1234}:        "1234B",
		{Bytes: 0}:           "0B",
	} {
		if got, _ := z.MarshalText(); string(got) != want {
			t.Errorf("%+v: got %q, want %q", z, got, want)
		}
	}
}

func TestTarget(t *testing.T) {
	for _, in := range []string{"latest", "next-stop", "19.6", "19.6.1", "19.10"} {
		var v Target
		if err := v.UnmarshalText([]byte(in)); err != nil || string(v) != in {
			t.Errorf("%q: %v", in, err)
		}
	}
	for _, in := range []string{"", "19", "v19.6", "19.6.1.2", "19.6-ee", "Latest"} {
		var v Target
		if err := v.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
}

func TestHopsPerRun(t *testing.T) {
	var h HopsPerRun
	if err := h.UnmarshalText([]byte("all")); err != nil || !h.All {
		t.Errorf("all: %+v %v", h, err)
	}
	if err := h.UnmarshalText([]byte("3")); err != nil || h.All || h.N != 3 {
		t.Errorf("3: %+v %v", h, err)
	}
	for _, in := range []string{"0", "-1", "two", "", "1.5"} {
		if err := h.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
}

func TestPaths(t *testing.T) {
	for _, in := range []string{"auto", "/var/opt/gitlab/backups", "/"} {
		var p HostPath
		if err := p.UnmarshalText([]byte(in)); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}
	for _, in := range []string{"", "backups", "./x", `C:\backups`} {
		var p HostPath
		if err := p.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
	for _, in := range []string{"file:", "file:rel.json", "/abs.json", "http://x"} {
		var d DatasetSource
		if err := d.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("dataset %q: want an error", in)
		}
	}
	if DatasetSource("auto").File() != "" {
		t.Error("auto has no file")
	}
}

func TestWebhook(t *testing.T) {
	for _, in := range []string{"", "https://ntfy.example/x", "http://10.0.0.1:8080/hook?token=a"} {
		var w WebhookURL
		if err := w.UnmarshalText([]byte(in)); err != nil {
			t.Errorf("%q: %v", in, err)
		}
	}
	for _, in := range []string{"ntfy.example", "ftp://x", "https://", "://x"} {
		var w WebhookURL
		if err := w.UnmarshalText([]byte(in)); err == nil {
			t.Errorf("%q: want an error", in)
		}
	}
}
