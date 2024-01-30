// httperror is a simple package to define error along with http error code.

package httperror

import "errors"

// HTTPError is a custom error types that implements error interface with http error code information.
type HTTPError struct {
	// The real error from lower layer. We can possibly log this error in a middleware.
	Err error
	// Message will be used as the error message to the user.
	Message string
	// Code is the http code of when error happens.
	Code int
}

// Error returns the string of real error.
func (h *HTTPError) Error() string {
	return h.Err.Error()
}

// Is overrides the implementation of errors.Is. If this function is implemented, then errors.Is will seek
// the internal h.Err.
func (h *HTTPError) Is(err error) bool {
	return errors.Is(h.Err, err)
}
