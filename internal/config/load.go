package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// MaxFileSize is the largest config file gluper reads.
const MaxFileSize = 1 << 20

// EnvConfigPath names the environment variable that selects the config file.
const EnvConfigPath = "GLUPER_CONFIG"

// SourceKind says which layer a value came from.
type SourceKind string

const (
	SourceDefault SourceKind = "default"
	SourceFile    SourceKind = "file"
	SourceEnv     SourceKind = "env"
	SourceFlag    SourceKind = "flag"
)

// Source records where a value (or a problem) came from.
type Source struct {
	Kind SourceKind `json:"kind"`
	Name string     `json:"name,omitempty"` // file path, variable or flag
	Line int        `json:"line,omitempty"` // file line, when known
}

func (s Source) String() string {
	switch s.Kind {
	case SourceFile:
		if s.Line > 0 {
			return s.Name + ":" + strconv.Itoa(s.Line)
		}
		return s.Name
	case SourceEnv:
		return "env " + s.Name
	case SourceFlag:
		return "flag " + s.Name
	default:
		return string(s.Kind)
	}
}

// Severity of a Problem.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Problem is one validation finding.
type Problem struct {
	Severity Severity `json:"severity"`
	Source   Source   `json:"source"`
	Key      string   `json:"key,omitempty"`
	Message  string   `json:"message"`
}

func (p Problem) String() string {
	var sb strings.Builder
	sb.WriteString(string(p.Severity))
	sb.WriteString(": ")
	if p.Source.Kind != "" && p.Source.Kind != SourceDefault {
		sb.WriteString(p.Source.String())
		sb.WriteString(": ")
	}
	if p.Key != "" {
		sb.WriteString(p.Key)
		sb.WriteString(": ")
	}
	sb.WriteString(p.Message)
	return sb.String()
}

// Override sets one key from a command-line flag. List keys take a
// comma-separated Value.
type Override struct {
	Key   string
	Value string
	Flag  string // such as "--target"
}

// Options control Load.
type Options struct {
	// Path is an explicit config file, usually from --config. When empty,
	// GLUPER_CONFIG and then DefaultPath are used.
	Path string
	// PathSource describes where Path came from, for messages
	// (such as "flag --config"). Optional.
	PathSource string
	// DefaultPath is used when no file is named explicitly. A missing default
	// file is not an error.
	DefaultPath string
	// Environ holds the environment as KEY=VALUE pairs (os.Environ()).
	Environ []string
	// Overrides come from command-line flags.
	Overrides []Override
	// RequireTrusted refuses a config file that someone other than root could
	// have written. Set it when running as root: hooks run as root.
	RequireTrusted bool

	// open replaces os.Open in tests.
	open func(string) (fs.File, error)
	// trust replaces checkTrusted in tests.
	trust func(string) error
}

// Loaded is the result of Load.
type Loaded struct {
	Config Config
	// File is the config file that was selected, read or not.
	File string
	// FileSource says why File was selected: "default", "env GLUPER_CONFIG",
	// or Options.PathSource.
	FileSource string
	// FileFound reports whether File existed and was read.
	FileFound bool
	// Sources maps every key path to where its effective value came from.
	Sources  map[string]Source
	Problems []Problem
}

// HasErrors reports whether any problem is an error.
func (l *Loaded) HasErrors() bool {
	for _, p := range l.Problems {
		if p.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Err returns an error summarizing the error-level problems, or nil.
func (l *Loaded) Err() error {
	var msgs []string
	for _, p := range l.Problems {
		if p.Severity == SeverityError {
			msgs = append(msgs, p.String())
		}
	}
	switch len(msgs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("invalid configuration: %s", strings.TrimPrefix(msgs[0], "error: "))
	default:
		return fmt.Errorf("invalid configuration (%d errors):\n  %s", len(msgs), strings.Join(msgs, "\n  "))
	}
}

// Load builds the effective configuration from defaults, the config file,
// the environment and flag overrides. It never fails outright: every problem,
// including an unreadable file, is reported in Loaded.Problems.
func Load(opts Options) *Loaded {
	l := &loader{
		out: &Loaded{
			Config:  Defaults(),
			Sources: make(map[string]Source, len(allKeys)),
		},
		opts: opts,
	}
	for _, k := range allKeys {
		l.out.Sources[k.Path] = Source{Kind: SourceDefault}
	}
	env := parseEnviron(opts.Environ)

	l.selectFile(env)
	l.loadFile()
	l.loadEnv(env)
	l.loadOverrides()
	l.out.Problems = append(l.out.Problems, check(l.out)...)
	return l.out
}

type loader struct {
	out  *Loaded
	opts Options
}

func (l *loader) problem(sev Severity, src Source, key, format string, args ...any) {
	l.out.Problems = append(l.out.Problems, Problem{
		Severity: sev, Source: src, Key: key, Message: fmt.Sprintf(format, args...),
	})
}

func parseEnviron(environ []string) map[string]string {
	env := map[string]string{}
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, "GLUPER_") {
			env[k] = v
		}
	}
	return env
}

func (l *loader) selectFile(env map[string]string) {
	switch {
	case l.opts.Path != "":
		l.out.File = l.opts.Path
		l.out.FileSource = l.opts.PathSource
		if l.out.FileSource == "" {
			l.out.FileSource = "explicit"
		}
	case env[EnvConfigPath] != "":
		l.out.File = env[EnvConfigPath]
		l.out.FileSource = "env " + EnvConfigPath
	default:
		l.out.File = l.opts.DefaultPath
		l.out.FileSource = "default"
	}
}

func (l *loader) loadFile() {
	name := l.out.File
	if name == "" {
		return
	}
	src := Source{Kind: SourceFile, Name: name}
	explicit := l.out.FileSource != "default"

	open := l.opts.open
	if open == nil {
		open = func(n string) (fs.File, error) {
			return os.Open(n) //nolint:gosec // G304: reading the config file the user named is the point
		}
	}
	f, err := open(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && !explicit {
			return // no config file: defaults apply
		}
		l.problem(SeverityError, src, "", "cannot read config file: %v", err)
		return
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil && !fi.Mode().IsRegular() {
		l.problem(SeverityError, src, "", "config file is not a regular file")
		return
	}
	if l.opts.RequireTrusted {
		trust := l.opts.trust
		if trust == nil {
			trust = checkTrusted
		}
		if err := trust(name); err != nil {
			l.problem(SeverityError, src, "", "refusing to use config file: %v", err)
			return
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		l.problem(SeverityError, src, "", "cannot read config file: %v", err)
		return
	}
	if len(data) > MaxFileSize {
		l.problem(SeverityError, src, "", "config file is larger than %d bytes", MaxFileSize)
		return
	}
	l.out.FileFound = true
	l.parseYAML(name, data)
}

var yamlLine = regexp.MustCompile(`^yaml: line (\d+): (.*)$`)

func (l *loader) parseYAML(name string, data []byte) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		src := Source{Kind: SourceFile, Name: name}
		msg := err.Error()
		if m := yamlLine.FindStringSubmatch(msg); m != nil {
			src.Line, _ = strconv.Atoi(m[1])
			msg = m[2]
		}
		l.problem(SeverityError, src, "", "invalid YAML: %s", strings.TrimPrefix(msg, "yaml: "))
		return
	}
	if len(doc.Content) == 0 {
		return // empty file
	}
	root := resolveAlias(doc.Content[0])
	if isNull(root) {
		return
	}
	if root.Kind != yaml.MappingNode {
		l.problem(SeverityError, Source{Kind: SourceFile, Name: name, Line: root.Line}, "",
			"the top level must be a mapping of keys to values")
		return
	}
	l.walk(name, root, "")
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n != nil && n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	return n
}

func isNull(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

func (l *loader) walk(name string, n *yaml.Node, prefix string) {
	seen := map[string]int{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		kn, vn := n.Content[i], resolveAlias(n.Content[i+1])
		src := Source{Kind: SourceFile, Name: name, Line: kn.Line}
		if kn.Kind != yaml.ScalarNode {
			l.problem(SeverityError, src, prefix, "keys must be plain names")
			continue
		}
		full := joinPath(prefix, kn.Value)
		if first, dup := seen[kn.Value]; dup {
			l.problem(SeverityError, src, full, "duplicate key (first set on line %d)", first)
			continue
		}
		seen[kn.Value] = kn.Line

		if isSection(full) {
			switch {
			case vn.Kind == yaml.MappingNode:
				l.walk(name, vn, full)
			case isNull(vn):
				// An empty section keeps its defaults.
			default:
				l.problem(SeverityError, src, full, "must be a section with keys inside it")
			}
			continue
		}
		key, ok := keyByPath[full]
		if !ok {
			l.unknownKey(src, prefix, kn.Value)
			continue
		}
		src.Line = vn.Line
		l.setFromNode(key, vn, src)
	}
}

func (l *loader) unknownKey(src Source, prefix, name string) {
	full := joinPath(prefix, name)
	if m, ok := reserved[full]; ok {
		l.problem(SeverityError, src, full, "is planned for milestone %s and not supported yet", m)
		return
	}
	msg := "unknown key"
	if s := suggest(prefix, name); s != "" {
		msg += fmt.Sprintf(" (did you mean %q?)", s)
	}
	l.problem(SeverityError, src, full, "%s", msg)
}

func (l *loader) setFromNode(key *Key, n *yaml.Node, src Source) {
	v := key.field(&l.out.Config)
	var err error
	if key.IsList() {
		switch {
		case isNull(n):
			err = setList(v, nil)
		case n.Kind != yaml.SequenceNode:
			err = errors.New("must be a list, such as [a, b]")
		default:
			items := make([]string, 0, len(n.Content))
			for _, it := range n.Content {
				it = resolveAlias(it)
				if it.Kind != yaml.ScalarNode {
					err = errors.New("list items must be plain values")
					break
				}
				items = append(items, it.Value)
			}
			if err == nil {
				err = setList(v, items)
			}
		}
	} else {
		switch {
		case n.Kind != yaml.ScalarNode:
			err = errors.New("must be a single value, not a list or section")
		case isNull(n):
			err = setText(v, "")
		default:
			err = setText(v, n.Value)
		}
	}
	if err != nil {
		l.problem(SeverityError, src, key.Path, "%v", err)
		return
	}
	l.out.Sources[key.Path] = src
}

func (l *loader) setFromString(key *Key, s string, src Source) {
	v := key.field(&l.out.Config)
	var err error
	if key.IsList() {
		err = setList(v, splitList(s))
	} else {
		err = setText(v, s)
	}
	if err != nil {
		l.problem(SeverityError, src, key.Path, "%v", err)
		return
	}
	l.out.Sources[key.Path] = src
}

// nonConfigEnv lists GLUPER_* variables that are not config keys.
var nonConfigEnv = map[string]bool{EnvConfigPath: true}

func (l *loader) loadEnv(env map[string]string) {
	used := map[string]bool{}
	for i := range allKeys {
		k := &allKeys[i]
		if v, ok := env[k.Env]; ok {
			used[k.Env] = true
			l.setFromString(k, v, Source{Kind: SourceEnv, Name: k.Env})
		}
	}
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if used[name] || nonConfigEnv[name] {
			continue
		}
		msg := "unknown gluper environment variable, ignored"
		if s := suggestEnv(name); s != "" {
			msg += fmt.Sprintf(" (did you mean %s?)", s)
		}
		l.problem(SeverityWarning, Source{Kind: SourceEnv, Name: name}, "", "%s", msg)
	}
}

func (l *loader) loadOverrides() {
	for _, o := range l.opts.Overrides {
		key, ok := keyByPath[o.Key]
		if !ok {
			// A programming error: flags are bound to keys in code.
			l.problem(SeverityError, Source{Kind: SourceFlag, Name: o.Flag}, o.Key, "flag is bound to an unknown config key")
			continue
		}
		l.setFromString(key, o.Value, Source{Kind: SourceFlag, Name: o.Flag})
	}
}

// Value is one effective setting, for display.
type Value struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Text   string `json:"-"`
	Source Source `json:"source"`
}

// Values lists every effective setting in schema order.
func (l *Loaded) Values() []Value {
	vals := make([]Value, 0, len(allKeys))
	for i := range allKeys {
		k := &allKeys[i]
		v := k.field(&l.Config)
		vals = append(vals, Value{
			Key:    k.Path,
			Value:  jsonValue(v),
			Text:   formatValue(v),
			Source: l.Sources[k.Path],
		})
	}
	return vals
}

// jsonValue returns v in a form that encodes to JSON the way it reads in the
// config file.
func jsonValue(v reflect.Value) any {
	if v.Kind() == reflect.Slice && v.IsNil() {
		return reflect.MakeSlice(v.Type(), 0, 0).Interface()
	}
	return v.Interface()
}
