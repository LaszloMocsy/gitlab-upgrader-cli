//go:build !linux

package cli

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/LaszloMocsy/gluper/internal/exitcode"
)

// hostCommands returns a stub for each host-only command, so that running one
// on macOS or Windows gives a clear answer instead of "unknown command".
func hostCommands() []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(hostCommandNames))
	for _, name := range hostCommandNames {
		cmds = append(cmds, unsupportedHostCommand(name))
	}
	return cmds
}

func unsupportedHostCommand(name string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Short:              "Runs only on the Linux host where GitLab is installed",
		DisableFlagParsing: true,
		RunE: func(*cobra.Command, []string) error {
			return exitcode.Newf(exitcode.Unsupported,
				"%q runs only on the Linux host where GitLab is installed; this machine is %s/%s",
				name, runtime.GOOS, runtime.GOARCH)
		},
	}
}
