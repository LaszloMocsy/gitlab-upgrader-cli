//go:build linux

package cli

import "github.com/spf13/cobra"

// hostCommands returns the commands that run on the GitLab host. Every name
// in hostCommandNames must have exactly one entry here.
func hostCommands() []*cobra.Command {
	return nil
}
