package repo

import "fmt"

// ErrInvalidID is an error about an invalid ID, either of a repository or media.
type ErrInvalidID struct {
	// ID is the offending id.
	ID string
	// Expected is the expected name form, a regular expression or a simple description.
	Expected string
}

// Error returns the string representation of the error.
func (ein *ErrInvalidID) Error() string {
	return fmt.Sprintf("invalid id %s, expected %s", ein.ID, ein.Expected)
}

// ErrInvalidMediaPath is an error about an unexpected media path,
// expected a path within the repository's root directory (could not relativize the media path).
type ErrInvalidMediaPath struct {
	// Path is the offending path.
	Path string
	// Root is the root directory of the repository.
	Root string
}

// Error returns the string representation of the error.
func (eimp *ErrInvalidMediaPath) Error() string {
	return fmt.Sprintf("invalid media path %s, outside of repository root %s", eimp.Path, eimp.Root)
}

// ErrInvalidMediaType is an error about an unexpected media MIME type.
type ErrInvalidMediaType struct {
	// Path is the offending media path.
	Path string
	// Type is the offending MIME type.
	Type string
}

// Error returns the string representation of the error.
func (eimt *ErrInvalidMediaType) Error() string {
	return fmt.Sprintf("invalid media MIME type %s, path %s", eimt.Type, eimt.Path)
}

// ErrDuplicateID is an error about a duplicate media ID in a repository.
type ErrDuplicateID struct {
	// ID is the offending ID.
	ID string
	// Repo is the repository name.
	Repo string
}

// Error returns the string representation of the error.
func (edi *ErrDuplicateID) Error() string {
	return fmt.Sprintf("duplicate media Name %s in repository %s", edi.ID, edi.Repo)
}

// ErrDuplicatePath is an error about a duplicate media path in a repository.
type ErrDuplicatePath struct {
	// Path is the offending path.
	Path string
	// Repo is the repository name.
	Repo string
}

// Error returns the string representation of the error.
func (edp *ErrDuplicatePath) Error() string {
	return fmt.Sprintf("duplicate media path %s in repository %s", edp.Path, edp.Repo)
}

// ErrUnsupportedFormat is an error about a format unsupported for de/muxing or transcoding.
type ErrUnsupportedFormat struct {
	// Format is the offending format name.
	Format string
	// Operation is the unsupported operation.
	Operation string
}

// Error returns the string representation of the error.
func (euf *ErrUnsupportedFormat) Error() string {
	return fmt.Sprintf("unsupported format %s for %s", euf.Format, euf.Operation)
}
