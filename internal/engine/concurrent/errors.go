package concurrent

import (
	"errors"
	"fmt"
)

// ErrFatal is an error that should cause the download to abort immediately.
// It is used for unrecoverable errors like 404 Not Found or 403 Forbidden.
var ErrFatal = errors.New("fatal download error")

// FatalError wraps an error and marks it as fatal.
type FatalError struct {
	Err error
}

func (e *FatalError) Error() string {
	return fmt.Sprintf("fatal error: %v", e.Err)
}

func (e *FatalError) Unwrap() error {
	return e.Err
}

// IsFatal checks if an error is a fatal error.
func IsFatal(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrFatal) {
		return true
	}
	var fatalErr *FatalError
	return errors.As(err, &fatalErr)
}

// NewFatalError creates a new FatalError wrapping the given error.
func NewFatalError(err error) error {
	return &FatalError{Err: err}
}
