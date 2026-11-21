// Package exitcode defines gluper's process exit codes. They are part of the
// public contract: scripts and cron jobs depend on them, so existing values
// never change meaning.
package exitcode

import (
	"errors"
	"fmt"
)

// Code is a process exit code.
type Code int

const (
	OK              Code = 0 // success
	Error           Code = 1 // generic error
	GuardBlocked    Code = 2 // a guard blocked the operation
	UpdateAvailable Code = 3 // versions --check: an update is available
	VerifyFailed    Code = 4 // post-upgrade verification failed
	Unsupported     Code = 5 // unsupported platform, install type or starting version
)

// ExitError carries an exit code alongside an error.
type ExitError struct {
	Code Code
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// New wraps err so that the process exits with code.
func New(code Code, err error) error {
	return &ExitError{Code: code, Err: err}
}

// Newf is New with a formatted message.
func Newf(code Code, format string, args ...any) error {
	return New(code, fmt.Errorf(format, args...))
}

// From returns the exit code for err: OK for nil, the wrapped code for an
// ExitError anywhere in the chain, and Error otherwise.
func From(err error) Code {
	if err == nil {
		return OK
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return Error
}
