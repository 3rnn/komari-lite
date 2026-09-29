package metric

import "errors"

var (
	// ErrInvalidArgument reports an invalid caller-supplied argument.
	//
	// ErrInvalidArgument indicates that the caller passed in invalid parameters.
	ErrInvalidArgument = errors.New("metric: invalid argument")
	// ErrNotFound reports a missing metric or record.
	//
	// ErrNotFound indicates that the metric or record does not exist.
	ErrNotFound = errors.New("metric: not found")
	// ErrNoData reports that a valid query range contained no samples.
	//
	// ErrNoData indicates that there are no samples within the valid query range.
	ErrNoData = errors.New("metric: no data in range")
	// ErrAlreadyExists reports that a create-only operation found an existing row.
	//
	// ErrAlreadyExists indicates that only the create operation encountered an existing row.
	ErrAlreadyExists = errors.New("metric: already exists")
	// ErrClosed reports that the store has already been closed.
	//
	// ErrClosed indicates that the Store has been closed.
	ErrClosed = errors.New("metric: store is closed")
)
