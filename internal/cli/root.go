// Package cli wires gluper's cobra commands.
package cli

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/LaszloMocsy/gluper/internal/buildinfo"
	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

// app holds everything commands take from the process, so tests can replace
// it.
type app struct {
	stdout, stderr io.Writer
	environ        []string
	goos           string
	euid           int // -1 on Windows
	userConfigDir  func() (string, error)

	// Global flags.
	configPath string
}

func newApp(stdout, stderr io.Writer) *app {
	return &app{
		stdout:        stdout,
		stderr:        stderr,
		environ:       os.Environ(),
		goos:          runtime.GOOS,
		euid:          os.Geteuid(),
		userConfigDir: os.UserConfigDir,
	}
}

// Execute runs gluper with args and returns the process exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	return execute(newApp(stdout, stderr), args)
}

func execute(a *app, args []string) int {
	root := newRootCommand(a)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		fmt.Fprintf(a.stderr, "gluper: %v\n", err)
	}
	return int(exitcode.From(err))
}

func newRootCommand(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   "gluper",
		Short: "Plan and run safe upgrades of self-managed GitLab",
		Long: `gluper inspects a self-managed GitLab installation, works out the correct
upgrade path, refuses unsafe upgrades, and runs upgrades that can be resumed.

gluper is unofficial and not affiliated with GitLab Inc.`,
		Version:       buildinfo.Get().Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("gluper {{.Version}}\n")
	root.SetOut(a.stdout)
	root.SetErr(a.stderr)
	root.PersistentFlags().StringVar(&a.configPath, "config", "",
		"config file (default: $GLUPER_CONFIG, else /etc/gluper/config.yaml as root on Linux, else the user config dir)")

	root.AddCommand(newVersionCommand())
	root.AddCommand(newConfigCommand(a))
	root.AddCommand(hostCommands()...)
	return root
}
