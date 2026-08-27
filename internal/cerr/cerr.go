// Package cerr defines the structured error object and stable exit codes used
// across the CLI. Errors are objects: code, message, hint, retryability, and a
// doc link. Exit codes 1-7 keep copr-cli meanings for compatibility; new codes
// start at 8.
package cerr

import (
	"errors"
	"fmt"
)

// Exit codes. Codes 1-7 mirror copr-cli for script and muscle-memory
// compatibility; 8+ are new.
const (
	ExitOK           = 0
	ExitGeneric      = 1
	ExitUsage        = 2
	ExitNoConfig     = 3
	ExitBuildFailed  = 4
	ExitTransport    = 5
	ExitConfig       = 6
	ExitAuth         = 7
	ExitNotFound     = 8
	ExitPermission   = 9
	ExitConflict     = 10
	ExitTimeout      = 11
	ExitDrift        = 12
	ExitPrecondition = 13
	ExitInterrupted  = 130
)

// Error is the structured error object emitted on failure.
type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	Retryable bool   `json:"retryable"`
	Resource  string `json:"resource,omitempty"`
	Docs      string `json:"docs,omitempty"`
	ExitCode  int    `json:"exit_code"`
	Err       error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// ExitCodeFor extracts a stable exit code from any error. Unwraps *Error;
// everything else maps to the generic failure code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var ce *Error
	if errors.As(err, &ce) {
		if ce.ExitCode != 0 {
			return ce.ExitCode
		}
		return ExitGeneric
	}
	return ExitGeneric
}

// New builds an Error with the given code, message, and exit code.
func New(code string, exitCode int, message string) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode}
}

// Wrap attaches an underlying error to an Error.
func (e *Error) Wrap(err error) *Error {
	e.Err = err
	return e
}

// WithHint sets a remediation hint.
func (e *Error) WithHint(h string) *Error { e.Hint = h; return e }

// WithResource sets the affected resource reference.
func (e *Error) WithResource(r string) *Error { e.Resource = r; return e }

// Common error constructors.
func Usage(message string) *Error {
	return New("usage_error", ExitUsage, message)
}

func NotFound(resource string) *Error {
	return New("not_found", ExitNotFound, "resource not found").WithResource(resource)
}

func Auth(message string) *Error {
	return New("auth_failed", ExitAuth, message)
}

func Config(message string) *Error {
	return New("config_error", ExitConfig, message)
}

func Transport(message string) *Error {
	return New("transport_error", ExitTransport, message)
}
