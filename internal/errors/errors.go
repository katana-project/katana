package errors

import "errors"

// ErrUnsupported is an alias for errors.ErrUnsupported.
var ErrUnsupported = errors.ErrUnsupported

// New redirects to the errors.New method.
func New(text string) error {
	return errors.New(text)
}

// Is redirects to the errors.Is method.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
