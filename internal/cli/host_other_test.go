//go:build !linux

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

func TestUnsupportedHostCommand(t *testing.T) {
	root := newRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	root.AddCommand(unsupportedHostCommand("fake-host-cmd"))
	root.SetArgs([]string{"fake-host-cmd", "--any-flag", "arg"})

	err := root.Execute()
	if got := exitcode.From(err); got != exitcode.Unsupported {
		t.Fatalf("exit code = %d, want %d (err: %v)", got, exitcode.Unsupported, err)
	}
	if !strings.Contains(err.Error(), "Linux host") {
		t.Errorf("message does not explain where to run: %q", err)
	}
}

func TestEveryHostCommandHasAStub(t *testing.T) {
	cmds := hostCommands()
	if len(cmds) != len(hostCommandNames) {
		t.Fatalf("got %d stubs for %d host commands", len(cmds), len(hostCommandNames))
	}
}
