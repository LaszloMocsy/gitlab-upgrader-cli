package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestFrom(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Code
	}{
		{"nil", nil, OK},
		{"plain error", errors.New("boom"), Error},
		{"exit error", New(GuardBlocked, errors.New("blocked")), GuardBlocked},
		{"wrapped exit error", fmt.Errorf("context: %w", Newf(Unsupported, "not on %s", "windows")), Unsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := From(tt.err); got != tt.want {
				t.Errorf("From() = %d, want %d", got, tt.want)
			}
		})
	}
}

// The numeric values are a public contract; this test fails if one changes.
func TestContractValues(t *testing.T) {
	want := map[Code]int{OK: 0, Error: 1, GuardBlocked: 2, UpdateAvailable: 3, VerifyFailed: 4, Unsupported: 5}
	for c, v := range want {
		if int(c) != v {
			t.Errorf("code %d changed, want %d", c, v)
		}
	}
}

func TestExitErrorUnwraps(t *testing.T) {
	base := errors.New("base")
	err := New(VerifyFailed, base)
	if !errors.Is(err, base) {
		t.Error("errors.Is did not find the wrapped error")
	}
	if err.Error() != "base" {
		t.Errorf("Error() = %q", err.Error())
	}
}
