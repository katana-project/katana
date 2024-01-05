package server

import "fmt"

// ErrDuplicateRepo is an error about a duplicate repository ID.
type ErrDuplicateRepo struct {
	// ID is the offending repository ID.
	ID string
	// Path is the offending repository path.
	Path string
}

// Error returns the string representation of the error.
func (edr *ErrDuplicateRepo) Error() string {
	return fmt.Sprintf("duplicate repository Name %s, path %s", edr.ID, edr.Path)
}
