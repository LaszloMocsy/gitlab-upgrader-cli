package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/LaszloMocsy/gluper/internal/config"
)

func newConfigCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Create, show and validate gluper's configuration",
		Long: `Create, show and validate gluper's configuration.

Values are layered, highest precedence first: command-line flags, GLUPER_*
environment variables, the config file, and built-in defaults.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newConfigInitCommand(a), newConfigShowCommand(a), newConfigValidateCommand(a))
	return cmd
}

// configOptions returns the Load options for this process. path, when set,
// overrides --config.
func (a *app) configOptions(path, pathSource string) config.Options {
	opts := config.Options{
		Environ:        a.environ,
		RequireTrusted: a.euid == 0,
	}
	if def, err := config.DefaultPath(a.goos, a.euid, a.userConfigDir); err == nil {
		opts.DefaultPath = def
	}
	switch {
	case path != "":
		opts.Path, opts.PathSource = path, pathSource
	case a.configPath != "":
		opts.Path, opts.PathSource = a.configPath, "flag --config"
	}
	return opts
}

// configFile returns the file Load would select, and why.
func (a *app) configFile() (path, source string) {
	l := config.Load(a.configOptions("", ""))
	return l.File, l.FileSource
}

func newConfigInitCommand(a *app) *cobra.Command {
	var write, force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Print an annotated config file with every default",
		Long: `Print an annotated config file with every default.

With --write, save it to the config file location instead (--config,
$GLUPER_CONFIG, or the default location) with mode 0600. An existing file is
only replaced with --force.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !write {
				_, err := cmd.OutOrStdout().Write(config.Template())
				return err
			}
			path, _ := a.configFile()
			if path == "" {
				return errors.New("no config file location: pass --config PATH")
			}
			if err := writeConfigFile(path, config.Template(), force); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\nCheck your edits with: gluper config validate\n", path)
			return err
		},
	}
	cmd.Flags().BoolVar(&write, "write", false, "write the file instead of printing it")
	cmd.Flags().BoolVar(&force, "force", false, "with --write, replace an existing file")
	return cmd
}

// writeConfigFile writes data to path with mode 0600, atomically. It refuses
// to replace an existing file unless force is set.
func writeConfigFile(path string, data []byte, force bool) error {
	if _, err := os.Lstat(path); err == nil && !force {
		return fmt.Errorf("%s already exists; pass --force to replace it", path)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	err = tmp.Chmod(0o600)
	if err == nil {
		_, err = tmp.Write(data)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func newConfigShowCommand(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print every effective setting and where it came from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			l := config.Load(a.configOptions("", ""))
			if l.HasErrors() {
				return l.Err()
			}
			for _, p := range l.Problems {
				fmt.Fprintln(cmd.ErrOrStderr(), p.String())
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), struct {
					File     configFileJSON   `json:"file"`
					Values   []config.Value   `json:"values"`
					Problems []config.Problem `json:"problems"`
				}{fileJSON(l), l.Values(), problemsOrEmpty(l.Problems)})
			}
			return writeConfigTable(cmd.OutOrStdout(), l)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

func writeConfigTable(w io.Writer, l *config.Loaded) error {
	if _, err := fmt.Fprintf(w, "Config file: %s\n\n", describeFile(l)); err != nil {
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KEY\tVALUE\tSOURCE")
	for _, v := range l.Values() {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", v.Key, v.Text, v.Source)
	}
	return tw.Flush()
}

func describeFile(l *config.Loaded) string {
	switch {
	case l.File == "":
		return "none (built-in defaults)"
	case !l.FileFound:
		return fmt.Sprintf("%s (%s; not found, using built-in defaults)", l.File, l.FileSource)
	default:
		return fmt.Sprintf("%s (%s)", l.File, l.FileSource)
	}
}

type configFileJSON struct {
	Path       string `json:"path"`
	SelectedBy string `json:"selected_by"`
	Found      bool   `json:"found"`
}

func fileJSON(l *config.Loaded) configFileJSON {
	return configFileJSON{Path: l.File, SelectedBy: l.FileSource, Found: l.FileFound}
}

func problemsOrEmpty(ps []config.Problem) []config.Problem {
	if ps == nil {
		return []config.Problem{}
	}
	return ps
}

func newConfigValidateCommand(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "validate [FILE]",
		Short: "Check the config file and GLUPER_* variables",
		Long: `Check the config file and GLUPER_* environment variables: unknown keys,
types, allowed values, and settings that conflict.

FILE defaults to the file gluper would read (--config, $GLUPER_CONFIG, or the
default location). Exits with code 1 when there are errors; warnings alone
don't fail.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 1 {
				path = args[0]
			}
			l := config.Load(a.configOptions(path, "argument"))
			if asJSON {
				if err := writeJSON(cmd.OutOrStdout(), struct {
					File     configFileJSON   `json:"file"`
					Valid    bool             `json:"valid"`
					Problems []config.Problem `json:"problems"`
				}{fileJSON(l), !l.HasErrors(), problemsOrEmpty(l.Problems)}); err != nil {
					return err
				}
			} else if err := writeValidation(cmd.OutOrStdout(), l); err != nil {
				return err
			}
			if l.HasErrors() {
				return errors.New("the configuration is invalid")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print machine-readable JSON")
	return cmd
}

func writeValidation(w io.Writer, l *config.Loaded) error {
	var errs, warns int
	for _, p := range l.Problems {
		if p.Severity == config.SeverityError {
			errs++
		} else {
			warns++
		}
		if _, err := fmt.Fprintln(w, p.String()); err != nil {
			return err
		}
	}
	subject, verb := l.File, "is"
	if !l.FileFound && !fileHasProblems(l) {
		subject, verb = "The built-in defaults and environment", "are"
		if l.File != "" && errs == 0 {
			fmt.Fprintf(w, "No config file at %s.\n", l.File)
		}
	}
	var msg string
	switch {
	case errs > 0:
		msg = fmt.Sprintf("%s: %s, %s.", subject, plural(errs, "error"), plural(warns, "warning"))
	case warns > 0:
		msg = fmt.Sprintf("%s %s valid, with %s.", subject, verb, plural(warns, "warning"))
	default:
		msg = fmt.Sprintf("%s %s valid.", subject, verb)
	}
	_, err := fmt.Fprintln(w, msg)
	return err
}

// fileHasProblems reports whether a problem is about the config file itself,
// such as one that could not be read or was refused.
func fileHasProblems(l *config.Loaded) bool {
	for _, p := range l.Problems {
		if p.Source.Kind == config.SourceFile && p.Source.Name == l.File {
			return true
		}
	}
	return false
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
