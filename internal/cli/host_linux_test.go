//go:build linux

package cli

import (
	"slices"
	"testing"
)

func TestHostCommandsMatchNames(t *testing.T) {
	var got []string
	for _, c := range hostCommands() {
		got = append(got, c.Name())
	}
	want := slices.Clone(hostCommandNames)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("host commands %v do not match hostCommandNames %v", got, want)
	}
}
