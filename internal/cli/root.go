// Package cli wires gluper's cobra commands.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LaszloMocsy/gluper/internal/buildinfo"
	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

// Execute runs gluper with args and returns the process exit code.
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)
	err := root.Execute()
	if err != nil {
		fmt.Fprintf(stderr, "gluper: %v\n", err)
	}
	return int(exitcode.From(err))
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
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
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newVersionCommand())
	root.AddCommand(hostCommands()...)
	return root
}
