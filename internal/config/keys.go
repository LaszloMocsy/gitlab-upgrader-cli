package config

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Key is one configurable leaf value, such as "backup.max_age".
type Key struct {
	Path  string // dotted YAML path
	Env   string // environment variable, such as GLUPER_BACKUP_MAX_AGE
	index []int  // field index path into Config
	typ   reflect.Type
}

// IsList reports whether the key holds a list of values.
func (k Key) IsList() bool {
	return k.typ.Kind() == reflect.Slice && !implementsText(k.typ)
}

func (k Key) field(cfg *Config) reflect.Value {
	return reflect.ValueOf(cfg).Elem().FieldByIndex(k.index)
}

// reserved lists keys that are documented for a later milestone. Using one
// gives a clear message instead of "unknown key".
var reserved = map[string]string{
	"install.compose_file": "M6",
	"install.service":      "M6",
}

var (
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
	textMarshalerType   = reflect.TypeFor[encoding.TextMarshaler]()

	// allKeys lists every key in schema order.
	allKeys = buildKeys(reflect.TypeFor[Config](), "", nil)
	// keyByPath indexes allKeys by Path.
	keyByPath = indexKeys(allKeys)
	// sections maps each section path ("" for the top level) to the names
	// directly inside it.
	sections = buildSections(allKeys)
)

// Keys returns every configurable key in schema order.
func Keys() []Key { return append([]Key(nil), allKeys...) }

func implementsText(t reflect.Type) bool {
	return reflect.PointerTo(t).Implements(textUnmarshalerType)
}

func buildKeys(t reflect.Type, prefix string, index []int) []Key {
	var keys []Key
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			panic(fmt.Sprintf("config: field %s.%s has no yaml name", t.Name(), f.Name))
		}
		p := joinPath(prefix, name)
		idx := append(append([]int(nil), index...), i)
		if f.Type.Kind() == reflect.Struct && !implementsText(f.Type) {
			keys = append(keys, buildKeys(f.Type, p, idx)...)
			continue
		}
		keys = append(keys, Key{
			Path:  p,
			Env:   "GLUPER_" + strings.ToUpper(strings.ReplaceAll(p, ".", "_")),
			index: idx,
			typ:   f.Type,
		})
	}
	return keys
}

func indexKeys(keys []Key) map[string]*Key {
	m := make(map[string]*Key, len(keys))
	for i := range keys {
		m[keys[i].Path] = &keys[i]
	}
	return m
}

func buildSections(keys []Key) map[string][]string {
	m := map[string][]string{}
	add := func(section, name string) {
		for _, n := range m[section] {
			if n == name {
				return
			}
		}
		m[section] = append(m[section], name)
	}
	for _, k := range keys {
		parts := strings.Split(k.Path, ".")
		for i := range parts {
			add(strings.Join(parts[:i], "."), parts[i])
		}
	}
	return m
}

func isSection(p string) bool {
	_, ok := sections[p]
	return ok && p != ""
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// setText parses s into the leaf v. v is only changed when parsing succeeds.
func setText(v reflect.Value, s string) error {
	tmp := reflect.New(v.Type())
	if u, ok := tmp.Interface().(encoding.TextUnmarshaler); ok {
		if err := u.UnmarshalText([]byte(s)); err != nil {
			return err
		}
		v.Set(tmp.Elem())
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		switch s {
		case "true":
			v.SetBool(true)
		case "false":
			v.SetBool(false)
		default:
			return fmt.Errorf("must be true or false, not %q", s)
		}
	case reflect.Int:
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("must be a whole number, not %q", s)
		}
		v.SetInt(int64(n))
	default:
		panic(fmt.Sprintf("config: unsupported leaf type %s", v.Type()))
	}
	return nil
}

// setList parses items into the list v. v is only changed when every item
// parses.
func setList(v reflect.Value, items []string) error {
	out := reflect.MakeSlice(v.Type(), len(items), len(items))
	for i, it := range items {
		if strings.TrimSpace(it) == "" {
			return fmt.Errorf("item %d is empty", i+1)
		}
		if err := setText(out.Index(i), it); err != nil {
			return fmt.Errorf("item %d %w", i+1, err)
		}
	}
	v.Set(out)
	return nil
}

// splitList splits a comma-separated environment or flag value.
func splitList(s string) []string {
	items := []string{}
	for it := range strings.SplitSeq(s, ",") {
		if it = strings.TrimSpace(it); it != "" {
			items = append(items, it)
		}
	}
	return items
}

// formatValue renders a leaf for humans, in the same syntax the config file
// accepts.
func formatValue(v reflect.Value) string {
	if v.Type().Implements(textMarshalerType) {
		b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return fmt.Sprintf("<%v>", err)
		}
		if len(b) == 0 {
			return `""`
		}
		return string(b)
	}
	switch v.Kind() {
	case reflect.Slice:
		items := make([]string, v.Len())
		for i := range items {
			items[i] = formatValue(v.Index(i))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case reflect.String:
		if v.String() == "" {
			return `""`
		}
		return v.String()
	default:
		return fmt.Sprint(v.Interface())
	}
}
