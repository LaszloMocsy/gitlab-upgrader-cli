package cli

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/LaszloMocsy/gluper/internal/buildinfo"
	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Execute(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersionText(t *testing.T) {
	code, out, _ := run(t, "version")
	if code != int(exitcode.OK) {
		t.Fatalf("exit code = %d", code)
	}
	for _, want := range []string{"gluper ", "commit", "platform  " + runtime.GOOS + "/" + runtime.GOARCH} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestVersionJSON(t *testing.T) {
	code, out, _ := run(t, "version", "--json")
	if code != int(exitcode.OK) {
		t.Fatalf("exit code = %d", code)
	}
	var info buildinfo.Info
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if info.Version == "" || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		t.Errorf("unexpected info: %+v", info)
	}
}

func TestVersionFlag(t *testing.T) {
	code, out, _ := run(t, "--version")
	if code != int(exitcode.OK) || !strings.HasPrefix(out, "gluper ") {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestWriteVersionText(t *testing.T) {
	var buf bytes.Buffer
	info := buildinfo.Info{Version: "1.2.3", Commit: "abc", Modified: true, GoVersion: "go1.27.1", OS: "linux", Arch: "arm64"}
	if err := writeVersion(&buf, info, false); err != nil {
		t.Fatal(err)
	}
	want := "gluper 1.2.3\n  commit    abc (modified)\n  built     unknown\n  go        go1.27.1\n  platform  linux/arm64\n"
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestUnknownCommandFails(t *testing.T) {
	code, _, errOut := run(t, "no-such-command")
	if code != int(exitcode.Error) {
		t.Errorf("exit code = %d, want %d", code, exitcode.Error)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestVersionRejectsArgs(t *testing.T) {
	if code, _, _ := run(t, "version", "extra"); code != int(exitcode.Error) {
		t.Errorf("exit code = %d, want %d", code, exitcode.Error)
	}
}
