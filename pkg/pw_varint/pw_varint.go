package pw_varint

import (
	"errors"
	"slices"
)

type Format int

const (
	ZeroTerminatedLeastSignificant Format = 0
	ZeroTerminatedMostSignificant  Format = 1
	OneTerminatedLeastSignificant  Format = 2
	OneTerminatedMostSignificant   Format = 3
	MaxVarint32SizeBytes                  = 5
	MaxVarint64SizeBytes                  = 10
)

var (
	ErrOutputTooSmall = errors.New("output too small")
)

/// Maximum size of a varint (LEB128) encoded `uint32_t`.

/// Maximum size of a varint (LEB128) encoded `uint64_t`.

func ZeroTerminated(format Format) bool {
	return (format & 0b10) == 0
}

func LeastSignificant(format Format) bool {
	return (format & 0b01) == 0
}

func If[T any](cond bool, vTrue, vFalse T) T {
	if cond {
		return vTrue
	}
	return vFalse
}

func Encode(
	val uint64,
	format Format,
) []byte {
	output := make([]byte, 10)
	written := 0

	value_shift := If[int](LeastSignificant(format), 1, 0)
	term_shift := If[int](value_shift == 1, 0, 7)

	cont := byte(0x00) << term_shift
	term := byte(0x01) << term_shift

	if ZeroTerminated(format) {
		cont = byte(0x01) << term_shift
		term = byte(0x00) << term_shift
	}

	for val != 0 {
		last_byte := (val >> 7) == 0

		// Grab 7 bits and set the eighth according to the continuation bit.
		value := (byte(val) & byte(0x7f)) << value_shift

		if last_byte {
			value |= term
		} else {
			value |= cont
		}

		output[written] = value
		written++
		val >>= 7
	}

	return output[0:written]
}

func Decode(input []byte, format Format) (uint64, int) {
	decoded_value := uint64(0)
	count := 0

	// The largest 64-bit ints require 10 B.
	max_count := slices.Min([]int{MaxVarint64SizeBytes, len(input)})

	var mask byte
	var shift uint32
	if LeastSignificant(format) {
		mask = byte(0xfe)
		shift = 1
	} else {
		mask = byte(0x7f)
		shift = 0
	}

	// Determines whether a byte is the last byte of a varint.
	is_last_byte := func(b byte) bool {
		if ZeroTerminated(format) {
			return (b & ^mask) == byte(0)
		}
		return (b & ^mask) != byte(0)
	}

	for {
		if count >= max_count {
			return 0, 0
		}

		// Add the bottom seven bits of the next byte to the result.
		val := (uint64((input[count] & mask) >> uint64(shift))) << (7 * count)
		decoded_value |= val

		// Stop decoding if the end is reached.
		if is_last_byte(input[count]) {
			break
		}
		count++
	}

	return decoded_value, count + 1
}
