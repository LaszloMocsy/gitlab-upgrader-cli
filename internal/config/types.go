package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Every leaf type below parses itself from text with UnmarshalText. The same
// parser serves YAML scalars (their raw text, so "19.10" never turns into the
// float 19.1), environment variables and flags.

// oneOf returns s if it is one of allowed. Values in planned are recognised
// but not available yet; the map value names the milestone that adds them.
func oneOf(s string, allowed []string, planned map[string]string) (string, error) {
	for _, a := range allowed {
		if s == a {
			return s, nil
		}
	}
	if m, ok := planned[s]; ok {
		return "", fmt.Errorf("%q is planned for milestone %s and not supported yet (use %s)", s, m, quoteList(allowed))
	}
	return "", fmt.Errorf("must be one of %s, not %q", quoteList(allowed), s)
}

func quoteList(items []string) string {
	q := make([]string, len(items))
	for i, it := range items {
		q[i] = strconv.Quote(it)
	}
	return strings.Join(q, ", ")
}

// Target is the version to upgrade to: latest, next-stop, MAJOR.MINOR or
// MAJOR.MINOR.PATCH.
type Target string

var targetVersion = regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)

func (t *Target) UnmarshalText(b []byte) error {
	s := string(b)
	if s != "latest" && s != "next-stop" && !targetVersion.MatchString(s) {
		return fmt.Errorf(`must be "latest", "next-stop", MAJOR.MINOR or MAJOR.MINOR.PATCH, not %q`, s)
	}
	*t = Target(s)
	return nil
}

// HopsPerRun is how many hops one run may do: a positive number, or all.
type HopsPerRun struct {
	All bool
	N   int
}

func (h *HopsPerRun) UnmarshalText(b []byte) error {
	s := string(b)
	if s == "all" {
		*h = HopsPerRun{All: true}
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return fmt.Errorf(`must be a whole number of at least 1, or "all", not %q`, s)
	}
	*h = HopsPerRun{N: n}
	return nil
}

func (h HopsPerRun) MarshalText() ([]byte, error) {
	if h.All {
		return []byte("all"), nil
	}
	return []byte(strconv.Itoa(h.N)), nil
}

func (h HopsPerRun) MarshalJSON() ([]byte, error) {
	if h.All {
		return []byte(`"all"`), nil
	}
	return json.Marshal(h.N)
}

// Checkpoint names an interactive pause during an upgrade.
type Checkpoint string

const (
	CheckpointBeforeBackup  Checkpoint = "before_backup"
	CheckpointBeforeInstall Checkpoint = "before_install"
)

func (c *Checkpoint) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{string(CheckpointBeforeBackup), string(CheckpointBeforeInstall)}, nil)
	*c = Checkpoint(s)
	return err
}

// InstallType is how GitLab is installed.
type InstallType string

func (v *InstallType) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"auto", "omnibus"}, map[string]string{"docker": "M6", "compose": "M6"})
	*v = InstallType(s)
	return err
}

// PackageManager selects the package manager adapter.
type PackageManager string

func (v *PackageManager) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"auto", "apt", "dnf", "yum", "zypper"}, nil)
	*v = PackageManager(s)
	return err
}

// Edition is the GitLab edition.
type Edition string

func (v *Edition) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"auto", "ce", "ee"}, nil)
	*v = Edition(s)
	return err
}

// PackageSource is where packages come from.
type PackageSource string

func (v *PackageSource) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"repo"}, map[string]string{"files": "M6"})
	*v = PackageSource(s)
	return err
}

// BackupStrategy is passed to gitlab-backup as STRATEGY.
type BackupStrategy string

func (v *BackupStrategy) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"default", "copy"}, nil)
	*v = BackupStrategy(s)
	return err
}

// VerifyArchive selects how a new backup is verified.
type VerifyArchive string

func (v *VerifyArchive) UnmarshalText(b []byte) error {
	s, err := oneOf(string(b), []string{"auto", "tar", "upload", "off"}, nil)
	*v = VerifyArchive(s)
	return err
}

// HostPath is "auto" or an absolute path on the GitLab host. Paths always use
// Linux semantics, even when the config is validated on macOS or Windows.
type HostPath string

func (p *HostPath) UnmarshalText(b []byte) error {
	s := string(b)
	if s != "auto" && !path.IsAbs(s) {
		return fmt.Errorf(`must be "auto" or an absolute path, not %q`, s)
	}
	*p = HostPath(s)
	return nil
}

// DatasetSource is "auto" (live, then cache, then the embedded snapshot) or
// "file:/absolute/path".
type DatasetSource string

func (d *DatasetSource) UnmarshalText(b []byte) error {
	s := string(b)
	if s != "auto" {
		p, ok := strings.CutPrefix(s, "file:")
		if !ok || !path.IsAbs(p) {
			return fmt.Errorf(`must be "auto" or "file:/absolute/path", not %q`, s)
		}
	}
	*d = DatasetSource(s)
	return nil
}

// File returns the dataset file path, or "" for auto.
func (d DatasetSource) File() string {
	p, _ := strings.CutPrefix(string(d), "file:")
	if p == string(d) {
		return ""
	}
	return p
}

// WebhookURL is empty (no notifications) or an http(s) URL.
type WebhookURL string

func (w *WebhookURL) UnmarshalText(b []byte) error {
	s := string(b)
	if s != "" {
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return fmt.Errorf("must be empty or an http(s) URL, not %q", s)
		}
	}
	*w = WebhookURL(s)
	return nil
}

// Duration is a time.Duration that also accepts whole days ("7d", "1d12h").
// A plain "0" means zero.
type Duration time.Duration

var leadingDays = regexp.MustCompile(`^(\d+)d`)

func (d *Duration) UnmarshalText(b []byte) error {
	s := string(b)
	if s == "0" {
		*d = 0
		return nil
	}
	if strings.HasPrefix(s, "-") {
		return fmt.Errorf("must not be negative, not %q", s)
	}
	bad := fmt.Errorf(`must be a duration such as "90m", "6h" or "7d", not %q`, s)
	var total time.Duration
	rest := s
	if m := leadingDays.FindStringSubmatch(s); m != nil {
		days, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || days > math.MaxInt64/int64(24*time.Hour) {
			return bad
		}
		total = time.Duration(days) * 24 * time.Hour
		rest = s[len(m[0]):]
	}
	if s == "" || strings.HasPrefix(rest, "-") || strings.HasPrefix(rest, "+") {
		return bad
	}
	if rest != "" {
		v, err := time.ParseDuration(rest)
		if err != nil {
			return bad
		}
		total += v
	}
	*d = Duration(total)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(formatDuration(time.Duration(d))), nil
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	day := 24 * time.Hour
	if d%day == 0 {
		return strconv.FormatInt(int64(d/day), 10) + "d"
	}
	if d%time.Second != 0 {
		return d.String()
	}
	var sb strings.Builder
	h, m, s := d/time.Hour, (d%time.Hour)/time.Minute, (d%time.Minute)/time.Second
	if h > 0 {
		fmt.Fprintf(&sb, "%dh", h)
	}
	if m > 0 {
		fmt.Fprintf(&sb, "%dm", m)
	}
	if s > 0 {
		fmt.Fprintf(&sb, "%ds", s)
	}
	return sb.String()
}

// Size is "auto" or an amount of bytes ("50GiB", "500MB", "1048576").
type Size struct {
	Auto  bool
	Bytes int64
}

var (
	sizeRe    = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([A-Za-z]*)$`)
	sizeUnits = map[string]int64{
		"":    1,
		"b":   1,
		"kb":  1000,
		"mb":  1000 * 1000,
		"gb":  1000 * 1000 * 1000,
		"tb":  1000 * 1000 * 1000 * 1000,
		"kib": 1 << 10,
		"mib": 1 << 20,
		"gib": 1 << 30,
		"tib": 1 << 40,
	}
	// sizeFormats lists units for printing, largest first, binary preferred.
	sizeFormats = []struct {
		name string
		mul  int64
	}{
		{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3},
	}
)

func (z *Size) UnmarshalText(b []byte) error {
	s := string(b)
	if s == "auto" {
		*z = Size{Auto: true}
		return nil
	}
	bad := fmt.Errorf(`must be "auto" or a size such as "50GiB" or "500MB", not %q`, s)
	m := sizeRe.FindStringSubmatch(s)
	if m == nil {
		return bad
	}
	mul, ok := sizeUnits[strings.ToLower(m[2])]
	if !ok {
		return bad
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return bad
	}
	bytes := n * float64(mul)
	if bytes > math.MaxInt64/2 {
		return errors.New("is too large")
	}
	*z = Size{Bytes: int64(math.Round(bytes))}
	return nil
}

func (z Size) MarshalText() ([]byte, error) {
	if z.Auto {
		return []byte("auto"), nil
	}
	if z.Bytes > 0 {
		for _, f := range sizeFormats {
			if z.Bytes%f.mul == 0 {
				return []byte(strconv.FormatInt(z.Bytes/f.mul, 10) + f.name), nil
			}
		}
	}
	return []byte(strconv.FormatInt(z.Bytes, 10) + "B"), nil
}
