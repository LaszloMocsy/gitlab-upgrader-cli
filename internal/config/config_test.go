package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// loadYAML loads src as the config file, plus optional environment entries.
func loadYAML(t *testing.T, src string, environ ...string) *Loaded {
	t.Helper()
	fsys := fstest.MapFS{"config.yaml": {Data: []byte(src)}}
	return Load(Options{
		Path:       "config.yaml",
		PathSource: "test",
		Environ:    environ,
		open:       fsys.Open,
	})
}

func errorsOf(l *Loaded) []string {
	var out []string
	for _, p := range l.Problems {
		if p.Severity == SeverityError {
			out = append(out, p.String())
		}
	}
	return out
}

func warningsOf(l *Loaded) []string {
	var out []string
	for _, p := range l.Problems {
		if p.Severity == SeverityWarning {
			out = append(out, p.String())
		}
	}
	return out
}

func wantOneProblem(t *testing.T, l *Loaded, sev Severity, contains ...string) {
	t.Helper()
	var got []string
	if sev == SeverityError {
		got = errorsOf(l)
	} else {
		got = warningsOf(l)
	}
	if len(got) != 1 {
		t.Fatalf("want exactly one %s containing %q, got %d: %q", sev, contains, len(got), got)
	}
	for _, c := range contains {
		if !strings.Contains(got[0], c) {
			t.Errorf("%s %q does not contain %q", sev, got[0], c)
		}
	}
}

func TestTemplateMatchesDefaults(t *testing.T) {
	l := loadYAML(t, string(Template()))
	if len(l.Problems) != 0 {
		t.Fatalf("template has problems: %q", l.Problems)
	}
	if !reflect.DeepEqual(l.Config, Defaults()) {
		t.Errorf("template does not parse to Defaults():\n got  %+v\n want %+v", l.Config, Defaults())
	}
	for _, k := range allKeys {
		if l.Sources[k.Path].Kind != SourceFile {
			t.Errorf("key %s is missing from template.yaml", k.Path)
		}
	}
}

func TestDefaultsAreValid(t *testing.T) {
	l := Load(Options{})
	if len(l.Problems) != 0 {
		t.Fatalf("defaults have problems: %q", l.Problems)
	}
	if l.FileFound {
		t.Error("no file was given, but FileFound is true")
	}
}

func TestKeysHaveUniqueEnvNames(t *testing.T) {
	seen := map[string]string{}
	for _, k := range allKeys {
		if other, dup := seen[k.Env]; dup {
			t.Errorf("%s and %s both map to %s", k.Path, other, k.Env)
		}
		seen[k.Env] = k.Path
		if nonConfigEnv[k.Env] {
			t.Errorf("%s collides with a non-config variable", k.Env)
		}
	}
}

func TestExampleProfilesAreValid(t *testing.T) {
	files, err := filepath.Glob("../../examples/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no example profiles found")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			l := Load(Options{Path: f})
			if !l.FileFound {
				t.Fatalf("example not read: %q", l.Problems)
			}
			if len(l.Problems) != 0 {
				t.Errorf("problems: %q", l.Problems)
			}
		})
	}
}

func TestFileValues(t *testing.T) {
	l := loadYAML(t, `
target: "19.10"
hops_per_run: all
checkpoints: [before_install]
install: { type: omnibus, package_manager: dnf, edition: ee }
backup:
  path: /mnt/backup/gitlab
  skip: [registry, artifacts]
  strategy: copy
  verify_archive: tar
  max_age: 1d12h
  min_free_space: 50GiB
  target:
    require_mount: true
    expected_fs: [nfs4, cifs]
config_backup: { dir: /mnt/backup/gitlab-config, strict: true }
dataset: { source: "file:/srv/dataset.json" }
upgrade: { bbm_wait_timeout: 90m, readiness_timeout: 5m, hold_package: false }
verify:
  smoke: { readiness: false }
  checklist: ["log in as a normal user", "open a merge request"]
hooks:
  pre_upgrade: ["/usr/local/bin/announce start"]
notify: { webhook: "https://ntfy.example/gitlab" }
maintenance_window: "Sat,Sun 02:00-06:00 Europe/Budapest"
`)
	if len(l.Problems) != 0 {
		t.Fatalf("problems: %q", l.Problems)
	}
	c := l.Config
	if c.Target != "19.10" {
		t.Errorf("target = %q (a YAML float would give 19.1)", c.Target)
	}
	if !c.HopsPerRun.All {
		t.Errorf("hops_per_run = %+v", c.HopsPerRun)
	}
	if !slices.Equal(c.Checkpoints, []Checkpoint{CheckpointBeforeInstall}) {
		t.Errorf("checkpoints = %v", c.Checkpoints)
	}
	if c.Install.PackageManager != "dnf" || c.Install.Edition != "ee" || c.Install.Type != "omnibus" {
		t.Errorf("install = %+v", c.Install)
	}
	if time.Duration(c.Backup.MaxAge) != 36*time.Hour {
		t.Errorf("max_age = %v", time.Duration(c.Backup.MaxAge))
	}
	if c.Backup.MinFreeSpace != (Size{Bytes: 50 << 30}) {
		t.Errorf("min_free_space = %+v", c.Backup.MinFreeSpace)
	}
	if c.Dataset.Source.File() != "/srv/dataset.json" {
		t.Errorf("dataset file = %q", c.Dataset.Source.File())
	}
	if len(c.Verify.Checklist) != 2 || c.Verify.Smoke.Readiness {
		t.Errorf("verify = %+v", c.Verify)
	}
	if got := l.Sources["backup.max_age"]; got.Kind != SourceFile || got.Line != 11 {
		t.Errorf("backup.max_age source = %+v, want file line 11", got)
	}
	if got := l.Sources["upgrade.hold_package"].String(); got != "config.yaml:18" {
		t.Errorf("upgrade.hold_package source = %q", got)
	}
	if got := l.Sources["notify.webhook"].Kind; got != SourceFile {
		t.Errorf("notify.webhook source kind = %q", got)
	}
}

func TestFileErrors(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		contains []string
	}{
		{"unknown key with suggestion", "backup:\n  maxage: 1h\n", []string{"config.yaml:2", "backup.maxage", `did you mean "backup.max_age"`}},
		{"key in wrong section", "max_age: 1h\n", []string{"max_age", `did you mean "backup.max_age"`}},
		{"unknown top-level key", "frobnicate: 1\n", []string{"frobnicate: unknown key"}},
		{"reserved for M6", "install:\n  compose_file: /srv/compose.yaml\n", []string{"install.compose_file", "milestone M6"}},
		{"planned enum value", "install: { type: compose }\n", []string{"install.type", "milestone M6"}},
		{"bad enum", "install: { edition: jh }\n", []string{"install.edition", `"auto", "ce", "ee"`}},
		{"bad bool", "backup: { enabled: yes }\n", []string{"backup.enabled", "true or false"}},
		{"bad duration", "backup: { max_age: soon }\n", []string{"backup.max_age", "duration"}},
		{"negative duration", "backup: { max_age: -1h }\n", []string{"negative"}},
		{"zero timeout", "upgrade: { readiness_timeout: 0 }\n", []string{"upgrade.readiness_timeout", "longer than 0"}},
		{"bad size", "backup: { min_free_space: lots }\n", []string{"backup.min_free_space"}},
		{"bad target", "target: 19\n", []string{"target", "MAJOR.MINOR"}},
		{"bad hops", "hops_per_run: 0\n", []string{"hops_per_run", "at least 1"}},
		{"relative path", "backup: { path: backups }\n", []string{"backup.path", "absolute"}},
		{"bad dataset", "dataset: { source: https://example.com/d.json }\n", []string{"dataset.source", "file:/absolute/path"}},
		{"bad webhook", "notify: { webhook: ftp://example.com }\n", []string{"notify.webhook"}},
		{"bad window", "maintenance_window: \"Caturday 02:00-06:00\"\n", []string{"maintenance_window", "unknown day"}},
		{"list expected", "checkpoints: before_backup\n", []string{"checkpoints", "must be a list"}},
		{"bad list item", "checkpoints: [before_backup, after_coffee]\n", []string{"checkpoints", "item 2", "after_coffee"}},
		{"empty list item", "verify: { checklist: [\"\"] }\n", []string{"verify.checklist", "item 1 is empty"}},
		{"scalar expected", "target: [latest]\n", []string{"target", "single value"}},
		{"section expected", "backup: true\n", []string{"backup", "section"}},
		{"duplicate key", "target: latest\ntarget: next-stop\n", []string{"config.yaml:2", "duplicate key", "line 1"}},
		{"syntax error", "backup:\n  path: [unclosed\n", []string{"invalid YAML"}},
		{"top level not a mapping", "- a\n- b\n", []string{"top level must be a mapping"}},
		{"skip tar", "backup: { skip: [tar] }\n", []string{"backup.skip", `"tar" is not allowed`}},
		{"upload verify without upload", "backup: { skip: [remote], verify_archive: upload }\n", []string{"backup.verify_archive", "remote"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantOneProblem(t, loadYAML(t, tt.yaml), SeverityError, tt.contains...)
		})
	}
}

func TestErrorLeavesDefault(t *testing.T) {
	l := loadYAML(t, "backup: { max_age: soon }\n")
	if l.Config.Backup.MaxAge != Defaults().Backup.MaxAge || l.Sources["backup.max_age"].Kind != SourceDefault {
		t.Errorf("a rejected value changed the config: %+v from %+v", l.Config.Backup.MaxAge, l.Sources["backup.max_age"])
	}
}

func TestWarnings(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		contains []string
	}{
		{"backups off", "backup: { enabled: false }\n", []string{"backup.enabled", "--skip-backup"}},
		{"verify off", "backup: { verify_archive: off }\n", []string{"not verified"}},
		{"skip db", "backup: { skip: [db] }\n", []string{`"db"`}},
		{"unknown skip", "backup: { skip: [widgets] }\n", []string{`"widgets"`, "unchanged"}},
		{"expected_fs without mount", "backup: { target: { expected_fs: [nfs4] } }\n", []string{"require_mount"}},
		{"strict with auto dir", "config_backup: { strict: true }\n", []string{"config_backup.dir"}},
		{"http webhook", "notify: { webhook: \"http://ntfy.local/x\" }\n", []string{"plain http"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := loadYAML(t, tt.yaml)
			if errs := errorsOf(l); len(errs) != 0 {
				t.Fatalf("unexpected errors: %q", errs)
			}
			wantOneProblem(t, l, SeverityWarning, tt.contains...)
		})
	}
}

func TestEmptyAndNullFile(t *testing.T) {
	for _, src := range []string{"", "# only a comment\n", "~\n", "backup:\n"} {
		l := loadYAML(t, src)
		if len(l.Problems) != 0 || !reflect.DeepEqual(l.Config, Defaults()) {
			t.Errorf("%q: problems %q", src, l.Problems)
		}
	}
}

func TestNullListMeansEmpty(t *testing.T) {
	l := loadYAML(t, "checkpoints:\n")
	if len(l.Problems) != 0 || len(l.Config.Checkpoints) != 0 || l.Config.Checkpoints == nil {
		t.Errorf("checkpoints = %#v, problems %q", l.Config.Checkpoints, l.Problems)
	}
}

func TestPrecedence(t *testing.T) {
	fsys := fstest.MapFS{"c.yaml": {Data: []byte("target: \"19.2\"\nhops_per_run: 2\nbackup: { max_age: 1h }\n")}}
	l := Load(Options{
		Path:    "c.yaml",
		Environ: []string{"GLUPER_HOPS_PER_RUN=3", "GLUPER_BACKUP_MAX_AGE=2h", "GLUPER_BACKUP_SKIP=registry, lfs", "PATH=/usr/bin"},
		Overrides: []Override{
			{Key: "backup.max_age", Value: "3h", Flag: "--max-age"},
		},
		open: fsys.Open,
	})
	if len(l.Problems) != 0 {
		t.Fatalf("problems: %q", l.Problems)
	}
	c := l.Config
	checks := []struct {
		key, want string
		got       any
		ok        bool
	}{
		{"target", "c.yaml:1", c.Target, c.Target == "19.2"},
		{"hops_per_run", "env GLUPER_HOPS_PER_RUN", c.HopsPerRun, c.HopsPerRun.N == 3},
		{"backup.max_age", "flag --max-age", c.Backup.MaxAge, time.Duration(c.Backup.MaxAge) == 3*time.Hour},
		{"backup.skip", "env GLUPER_BACKUP_SKIP", c.Backup.Skip, slices.Equal(c.Backup.Skip, []string{"registry", "lfs"})},
		{"install.type", "default", c.Install.Type, c.Install.Type == "auto"},
	}
	for _, ch := range checks {
		if !ch.ok {
			t.Errorf("%s = %v", ch.key, ch.got)
		}
		if got := l.Sources[ch.key].String(); got != ch.want {
			t.Errorf("%s source = %q, want %q", ch.key, got, ch.want)
		}
	}
}

func TestEnvProblems(t *testing.T) {
	l := Load(Options{Environ: []string{"GLUPER_BACKUP_MAX_AGES=1h", "GLUPER_CONFIG=", "GLUPER_HOPS_PER_RUN=many"}})
	wantOneProblem(t, l, SeverityWarning, "env GLUPER_BACKUP_MAX_AGES", "did you mean GLUPER_BACKUP_MAX_AGE?")
	wantOneProblem(t, l, SeverityError, "env GLUPER_HOPS_PER_RUN", "hops_per_run")
}

func TestEnvEmptyListClears(t *testing.T) {
	l := Load(Options{Environ: []string{"GLUPER_CHECKPOINTS="}})
	if len(l.Problems) != 0 || len(l.Config.Checkpoints) != 0 {
		t.Errorf("checkpoints = %v, problems %q", l.Config.Checkpoints, l.Problems)
	}
}

func TestFileSelection(t *testing.T) {
	fsys := fstest.MapFS{
		"flag.yaml":    {Data: []byte("target: \"1.1\"\n")},
		"env.yaml":     {Data: []byte("target: \"2.2\"\n")},
		"default.yaml": {Data: []byte("target: \"3.3\"\n")},
	}
	env := []string{"GLUPER_CONFIG=env.yaml"}
	tests := []struct {
		opts       Options
		wantTarget Target
		wantSource string
	}{
		{Options{Path: "flag.yaml", PathSource: "flag --config", Environ: env, DefaultPath: "default.yaml"}, "1.1", "flag --config"},
		{Options{Environ: env, DefaultPath: "default.yaml"}, "2.2", "env GLUPER_CONFIG"},
		{Options{DefaultPath: "default.yaml"}, "3.3", "default"},
	}
	for _, tt := range tests {
		tt.opts.open = fsys.Open
		l := Load(tt.opts)
		if l.Config.Target != tt.wantTarget || l.FileSource != tt.wantSource || !l.FileFound {
			t.Errorf("got target %q from %q (found %v), want %q from %q", l.Config.Target, l.FileSource, l.FileFound, tt.wantTarget, tt.wantSource)
		}
	}
}

func TestMissingFile(t *testing.T) {
	fsys := fstest.MapFS{}
	l := Load(Options{DefaultPath: "nope.yaml", open: fsys.Open})
	if len(l.Problems) != 0 || l.FileFound {
		t.Errorf("a missing default file must be fine: %q", l.Problems)
	}
	l = Load(Options{Path: "nope.yaml", PathSource: "flag --config", open: fsys.Open})
	wantOneProblem(t, l, SeverityError, "nope.yaml", "cannot read")
	l = Load(Options{Environ: []string{"GLUPER_CONFIG=nope.yaml"}, open: fsys.Open})
	wantOneProblem(t, l, SeverityError, "nope.yaml", "cannot read")
}

func TestFileTooLarge(t *testing.T) {
	big := strings.Repeat("# padding\n", MaxFileSize/10+1)
	wantOneProblem(t, loadYAML(t, big), SeverityError, "larger than")
}

func TestUntrustedFileRefused(t *testing.T) {
	fsys := fstest.MapFS{"c.yaml": {Data: []byte("target: next-stop\n")}}
	l := Load(Options{
		Path:           "c.yaml",
		RequireTrusted: true,
		open:           fsys.Open,
		trust:          func(string, fs.FileInfo) error { return errors.New("c.yaml is writable by group or others") },
	})
	wantOneProblem(t, l, SeverityError, "refusing to use config file", "writable")
	if l.Config.Target != "latest" {
		t.Error("an untrusted file was applied")
	}
}

func TestRealFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("target: next-stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := Load(Options{Path: p})
	if !l.FileFound || l.Config.Target != "next-stop" || len(l.Problems) != 0 {
		t.Errorf("got %+v, problems %q", l.Config.Target, l.Problems)
	}
	l = Load(Options{Path: dir})
	wantOneProblem(t, l, SeverityError, "not a regular file")
}

func TestDefaultPath(t *testing.T) {
	userDir := func() (string, error) { return filepath.Join("home", "u", ".config"), nil }
	if p, _ := DefaultPath("linux", 0, userDir); p != SystemPath {
		t.Errorf("root on linux: %q", p)
	}
	want := filepath.Join("home", "u", ".config", "gluper", "config.yaml")
	for _, c := range []struct {
		goos string
		euid int
	}{{"linux", 1000}, {"darwin", 0}, {"windows", -1}} {
		if p, _ := DefaultPath(c.goos, c.euid, userDir); p != want {
			t.Errorf("%s/%d: %q", c.goos, c.euid, p)
		}
	}
	if _, err := DefaultPath("darwin", 501, func() (string, error) { return "", errors.New("no HOME") }); err == nil {
		t.Error("want an error without a user config dir")
	}
}

func TestValuesJSON(t *testing.T) {
	l := Load(Options{})
	b, err := json.Marshal(l.Values())
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"hops_per_run":             float64(1),
		"upgrade.bbm_wait_timeout": "6h",
		"backup.min_free_space":    "auto",
		"backup.skip":              []any{},
		"maintenance_window":       "",
		"backup.enabled":           true,
	}
	for _, v := range got {
		if w, ok := want[v["key"].(string)]; ok && !reflect.DeepEqual(v["value"], w) {
			t.Errorf("%s = %#v, want %#v", v["key"], v["value"], w)
		}
	}
	if len(got) != len(allKeys) {
		t.Errorf("got %d values for %d keys", len(got), len(allKeys))
	}
}

func TestValuesText(t *testing.T) {
	texts := map[string]string{}
	for _, v := range Load(Options{}).Values() {
		texts[v.Key] = v.Text
	}
	for key, want := range map[string]string{
		"checkpoints":              "[before_backup, before_install]",
		"backup.skip":              "[]",
		"backup.max_age":           "0",
		"upgrade.bbm_wait_timeout": "6h",
		"notify.webhook":           `""`,
		"maintenance_window":       `""`,
		"hops_per_run":             "1",
	} {
		if texts[key] != want {
			t.Errorf("%s = %q, want %q", key, texts[key], want)
		}
	}
}

func TestErrSummary(t *testing.T) {
	if err := Load(Options{}).Err(); err != nil {
		t.Errorf("defaults: %v", err)
	}
	err := loadYAML(t, "target: 1\nhops_per_run: 0\n").Err()
	if err == nil || !strings.Contains(err.Error(), "2 errors") {
		t.Errorf("got %v", err)
	}
}
