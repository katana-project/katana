package meta

import "fmt"

// ErrInvalidQuery is an error about an invalid metadata query, most likely missing/unexpected data.
type ErrInvalidQuery struct {
	// Query is the string query.
	Query string
	// Type is the targeted metadata type.
	Type Type
}

// Error returns the string representation of the error.
func (eiq *ErrInvalidQuery) Error() string {
	return fmt.Sprintf("invalid metadata query %s of type %d", eiq.Query, eiq.Type)
}
