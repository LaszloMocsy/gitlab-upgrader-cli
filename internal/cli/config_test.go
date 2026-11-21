package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LaszloMocsy/gluper/internal/config"
	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

// testApp returns an app with an empty environment and a temporary user
// config directory, running as a normal user.
func testApp(t *testing.T, environ ...string) (*app, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	dir := t.TempDir()
	return &app{
		stdout:        &out,
		stderr:        &errOut,
		environ:       environ,
		goos:          runtime.GOOS,
		euid:          1000,
		userConfigDir: func() (string, error) { return dir, nil },
	}, &out, &errOut
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestConfigInitPrintsTemplate(t *testing.T) {
	a, out, _ := testApp(t)
	if code := execute(a, []string{"config", "init"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !bytes.Equal(out.Bytes(), config.Template()) {
		t.Error("output differs from the template")
	}
}

func TestConfigInitWrite(t *testing.T) {
	a, out, errOut := testApp(t)
	target := filepath.Join(t.TempDir(), "sub", "config.yaml")

	if code := execute(a, []string{"--config", target, "config", "init", "--write"}); code != 0 {
		t.Fatalf("exit code %d: %s", code, errOut)
	}
	if !strings.Contains(out.String(), "Wrote "+target) {
		t.Errorf("stdout = %q", out)
	}
	data, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(data, config.Template()) {
		t.Fatalf("file not written correctly: %v", err)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(target); fi.Mode().Perm() != 0o600 {
			t.Errorf("mode = %v, want 0600", fi.Mode().Perm())
		}
	}

	// A second write must not replace the file without --force.
	if err := os.WriteFile(target, []byte("target: next-stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	errOut.Reset()
	if code := execute(a, []string{"--config", target, "config", "init", "--write"}); code != int(exitcode.Error) {
		t.Fatalf("exit code %d, want %d", code, exitcode.Error)
	}
	if !strings.Contains(errOut.String(), "--force") {
		t.Errorf("stderr = %q", errOut)
	}
	if data, _ := os.ReadFile(target); string(data) != "target: next-stop\n" {
		t.Error("existing file was replaced without --force")
	}
	if code := execute(a, []string{"--config", target, "config", "init", "--write", "--force"}); code != 0 {
		t.Fatalf("--force: exit code %d", code)
	}
	if data, _ := os.ReadFile(target); !bytes.Equal(data, config.Template()) {
		t.Error("--force did not replace the file")
	}
}

func TestConfigInitWriteDefaultLocation(t *testing.T) {
	a, _, errOut := testApp(t)
	if code := execute(a, []string{"config", "init", "--write"}); code != 0 {
		t.Fatalf("exit code %d: %s", code, errOut)
	}
	dir, _ := a.userConfigDir()
	if _, err := os.Stat(filepath.Join(dir, "gluper", "config.yaml")); err != nil {
		t.Errorf("default file not written: %v", err)
	}
}

func TestConfigShowText(t *testing.T) {
	p := writeFile(t, "target: next-stop\n")
	a, out, errOut := testApp(t, "GLUPER_HOPS_PER_RUN=all", "GLUPER_TYPO=1")
	if code := execute(a, []string{"--config", p, "config", "show"}); code != 0 {
		t.Fatalf("exit code %d: %s", code, errOut)
	}
	got := out.String()
	for _, want := range []string{
		"Config file: " + p + " (flag --config)",
		"KEY", "SOURCE",
		"next-stop", p + ":1",
		"hops_per_run", "env GLUPER_HOPS_PER_RUN",
		"upgrade.bbm_wait_timeout", "6h",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if !strings.Contains(errOut.String(), "warning: env GLUPER_TYPO") {
		t.Errorf("warning not on stderr: %q", errOut)
	}
}

func TestConfigShowJSON(t *testing.T) {
	a, out, errOut := testApp(t)
	if code := execute(a, []string{"config", "show", "--json"}); code != 0 {
		t.Fatalf("exit code %d: %s", code, errOut)
	}
	var got struct {
		File struct {
			Path       string `json:"path"`
			SelectedBy string `json:"selected_by"`
			Found      bool   `json:"found"`
		} `json:"file"`
		Values []struct {
			Key    string        `json:"key"`
			Value  any           `json:"value"`
			Source config.Source `json:"source"`
		} `json:"values"`
		Problems []any `json:"problems"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.File.SelectedBy != "default" || got.File.Found || len(got.Values) != len(config.Keys()) || got.Problems == nil {
		t.Errorf("unexpected: %+v", got)
	}
	if got.Values[0].Key != "target" || got.Values[0].Value != "latest" || got.Values[0].Source.Kind != config.SourceDefault {
		t.Errorf("first value = %+v", got.Values[0])
	}
}

func TestConfigShowRefusesInvalidConfig(t *testing.T) {
	p := writeFile(t, "target: someday\n")
	a, out, errOut := testApp(t)
	if code := execute(a, []string{"--config", p, "config", "show"}); code != int(exitcode.Error) {
		t.Fatalf("exit code %d", code)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "target") {
		t.Errorf("stdout %q, stderr %q", out, errOut)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		environ  []string
		wantCode exitcode.Code
		want     []string
	}{
		{"valid", "target: next-stop\n", nil, exitcode.OK, []string{"is valid."}},
		{"warnings only", "backup: { enabled: false }\n", nil, exitcode.OK, []string{"warning:", "valid, with 1 warning."}},
		{"errors", "backup:\n  maxage: 1h\n", []string{"GLUPER_HOPS_PER_RUN=0"}, exitcode.Error,
			[]string{":2: backup.maxage: unknown key", "env GLUPER_HOPS_PER_RUN", "2 errors, 0 warnings."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeFile(t, tt.content)
			a, out, _ := testApp(t, tt.environ...)
			if code := execute(a, []string{"config", "validate", p}); code != int(tt.wantCode) {
				t.Fatalf("exit code %d, want %d\n%s", code, tt.wantCode, out)
			}
			for _, w := range tt.want {
				if !strings.Contains(out.String(), w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestConfigValidateNoFile(t *testing.T) {
	a, out, _ := testApp(t)
	if code := execute(a, []string{"config", "validate"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "No config file at") || !strings.Contains(out.String(), "are valid.") {
		t.Errorf("output: %s", out)
	}
}

func TestConfigValidateMissingExplicitFile(t *testing.T) {
	a, out, _ := testApp(t)
	missing := filepath.Join(t.TempDir(), "nope.yaml")
	if code := execute(a, []string{"config", "validate", missing}); code != int(exitcode.Error) {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "cannot read config file") || !strings.Contains(out.String(), missing+": 1 error") {
		t.Errorf("output: %s", out)
	}
}

func TestConfigValidateJSON(t *testing.T) {
	p := writeFile(t, "install: { type: docker }\n")
	a, out, _ := testApp(t)
	if code := execute(a, []string{"config", "validate", "--json", p}); code != int(exitcode.Error) {
		t.Fatalf("exit code %d", code)
	}
	var got struct {
		Valid    bool             `json:"valid"`
		Problems []config.Problem `json:"problems"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.Valid || len(got.Problems) != 1 || got.Problems[0].Key != "install.type" || got.Problems[0].Source.Line != 1 {
		t.Errorf("unexpected: %+v", got)
	}
}

func TestConfigEnvSelectsFile(t *testing.T) {
	p := writeFile(t, "target: next-stop\n")
	a, out, _ := testApp(t, "GLUPER_CONFIG="+p)
	if code := execute(a, []string{"config", "show"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "(env GLUPER_CONFIG)") {
		t.Errorf("output: %s", out)
	}
}

func TestConfigBareCommandShowsHelp(t *testing.T) {
	a, out, _ := testApp(t)
	if code := execute(a, []string{"config"}); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	for _, sub := range []string{"init", "show", "validate"} {
		if !strings.Contains(out.String(), sub) {
			t.Errorf("help missing %q", sub)
		}
	}
}
