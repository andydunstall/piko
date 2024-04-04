package status

import (
	"fmt"
	"net/http"
	"strings"
)

// ErrorInfo contains error information to be returned to the user. The
// contents of the error MUST only contain user visible state, never internal
// details.
type ErrorInfo struct {
	// StatusCode contains the HTTP status code.
	StatusCode int

	// Message contains the error message to return to the user.
	Message string
}

func (e *ErrorInfo) Error() string {
	return fmt.Sprintf(
		"%s (%d): %s",
		strings.ToLower(http.StatusText(e.StatusCode)),
		e.StatusCode,
		e.Message,
	)
}
