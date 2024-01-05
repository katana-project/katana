package errors

import (
	"fmt"
)

// Wrap wraps an error with a message.
func Wrap(err error, msg string) error {
	return fmt.Errorf("%s: %w", msg, err)
}

// Wrapf wraps an error with a formatted message.
func Wrapf(err error, format string, a ...interface{}) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), err)
}
