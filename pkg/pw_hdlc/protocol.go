package pw_hdlc

import "errors"

const (
	kFlag           = byte(0x7E)
	kEscape         = byte(0x7D)
	kEscapeConstant = byte(0x20)
	kUnusedControl  = byte(0)
	kUFramePattern  = byte(0x03)

	kControlSize         = 1
	kFcsSize             = 4
	kMinContentSizeBytes = 6
)

var (
	kEscapedFlag      = []byte{kEscape, 0x5E}
	kEscapedEscape    = []byte{kEscape, 0x5D}
	kUnnumberedUFrame = kUFramePattern
)

type State int

const (
	kInterFrame State = iota
	kFrame
	kFrameEscape
)

var (
	ErrDataLoss          = errors.New("data loss")
	ErrUnavailable       = errors.New("unavailable")
	ErrResourceExhausted = errors.New("resource exhausted")
	ErrInvalidArgument   = errors.New("invalid argument")
)

func NeedsEscaping(b byte) bool {
	return b == kFlag || b == kEscape
}
