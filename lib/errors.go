package lib

import "github.com/cockroachdb/errors"

var (
	ErrParamsNil         = errors.New("params cannot be nil")
	ErrParamsEmptyString = errors.New("params cannot be empty string")
)
