package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/LaszloMocsy/gluper/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print gluper's version and build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return writeVersion(cmd.OutOrStdout(), buildinfo.Get(), asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

func writeVersion(w io.Writer, info buildinfo.Info, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	commit := orUnknown(info.Commit)
	if info.Modified {
		commit += " (modified)"
	}
	_, err := fmt.Fprintf(w, "gluper %s\n  commit    %s\n  built     %s\n  go        %s\n  platform  %s/%s\n",
		info.Version, commit, orUnknown(info.Date), info.GoVersion, info.OS, info.Arch)
	return err
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
