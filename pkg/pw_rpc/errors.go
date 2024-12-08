package pw_rpc

import "errors"

var (
	ErrCancelled  = errors.New("cancelled")
	ErrBadAddress = errors.New("bad address")
)
