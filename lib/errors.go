package lib

import "github.com/cockroachdb/errors"

// エラー定数
var (
	ErrParamsNil = errors.New("params cannot be nil")
)
