package v1

// makeOptString converts a string to its pointer if it's not a zero value.
func makeOptString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// makeOptArray converts an array to its pointer if it's not empty.
func makeOptArray[T any](a []T) *[]T {
	if len(a) == 0 {
		return nil
	}
	return &a
}
