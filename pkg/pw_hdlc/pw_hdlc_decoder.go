package pw_hdlc

import (
	"encoding/binary"
	"hash/crc32"
	"io"
	"sync"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_varint"
)

type Decoder interface {
	Decode() (*Frame, error)
}

func NewDecoder(reader io.Reader, address uint64) Decoder {
	return &decoder{
		reader:            reader,
		address:           address,
		buffer:            make([]byte, 0),
		state:             kInterFrame,
		currentFrameSize:  0,
		lastReadBytes:     make([]byte, 4),
		lastReadByteIndex: 0,
		fcs:               0,
	}
}

type decoder struct {
	reader            io.Reader
	address           uint64
	buffer            []byte
	state             State
	currentFrameSize  int
	lastReadBytes     []byte
	lastReadByteIndex int
	fcs               uint32
	mu                sync.Mutex
}

func (d *decoder) Decode() (frame *Frame, err error) {
	d.mu.Lock()

	buf := make([]byte, 1)

	for {
		_, err = d.reader.Read(buf)
		if err != nil {
			break
		}
		frame, err = d.process(buf[0])
		if err == ErrUnavailable {
			continue
		} else if err != nil {
			break
		}

		break
	}

	d.reset()

	d.mu.Unlock()

	return frame, err
}

func (d *decoder) parse(frame []byte) (*Frame, error) {
	address, addressSize := pw_varint.Decode(frame, pw_varint.OneTerminatedLeastSignificant)

	dataSize := len(frame) - addressSize - kControlSize - kFcsSize

	if addressSize < 1 || dataSize < 0 {
		return nil, ErrDataLoss
	}

	start := addressSize + 1
	end := start + dataSize

	return NewFrame(address, frame[addressSize], frame[start:end]), nil
}

func (d *decoder) reset() {
	d.lastReadBytes = make([]byte, 4)
	d.currentFrameSize = 0
	d.lastReadByteIndex = 0
	d.fcs = 0
	d.buffer = make([]byte, 0)
}

func (d *decoder) escape(b byte) byte {
	return b ^ kEscapeConstant
}

func (d *decoder) process(newByte byte) (frame *Frame, err error) {
	switch d.state {
	case kInterFrame:
		if newByte == kFlag {
			d.state = kFrame

			// Report an error if non-flag bytes were read between frames.
			if d.currentFrameSize != 0 {
				err = ErrDataLoss
				break
			}
		} else {
			// Count bytes to track how many are discarded.
			d.currentFrameSize += 1
		}

		err = ErrUnavailable // Report error when starting a new frame.
	case kFrame:
		if newByte == kFlag {
			err = d.checkFrame()

			completedFrameSize := d.currentFrameSize

			if err == nil {
				frame, err = d.parse(d.buffer[0:completedFrameSize])
			}

			break
		}

		if newByte == kEscape {
			d.state = kFrameEscape
		} else {
			d.appendByte(newByte)
		}

		err = ErrUnavailable
	case kFrameEscape:
		// The flag character cannot be escaped; return an error.
		if newByte == kFlag {
			d.state = kFrame
			err = ErrDataLoss

			break
		}

		if newByte == kEscape {
			// Two escape characters in a row is illegal -- invalidate this frame.
			// The frame is reported abandoned when the next flag byte appears.
			d.state = kInterFrame

			// Count the escape byte so that the inter-frame state detects an error.
			d.currentFrameSize += 1
		} else {
			d.state = kFrame
			d.appendByte(d.escape(newByte))
		}

		err = ErrUnavailable
	}

	return frame, err
}

func (d *decoder) appendByte(newByte byte) {
	d.buffer = append(d.buffer, newByte)

	if d.currentFrameSize >= len(d.lastReadBytes) {
		// A byte will be ejected. Add it to the running checksum.
		ejectByte := d.lastReadBytes[d.lastReadByteIndex]
		d.fcs = crc32.Update(d.fcs, crc32.IEEETable, []byte{ejectByte})
	}

	d.lastReadBytes[d.lastReadByteIndex] = newByte
	d.lastReadByteIndex = (d.lastReadByteIndex + 1) % len(d.lastReadBytes)

	// Always increase size: if it is larger than the buffer, overflow occurred.
	d.currentFrameSize += 1
}

func (d *decoder) checkFrame() error {
	// Empty frames are not an error; repeated flag characters are okay.
	if d.currentFrameSize == 0 {
		return ErrUnavailable
	}

	if d.currentFrameSize < kMinContentSizeBytes {
		return ErrDataLoss
	}

	if !d.verifyFrameCheckSequence() {
		return ErrDataLoss
	}

	if d.currentFrameSize > len(d.buffer) {
		// Frame does not fit into the provided buffer; indicate this to the caller.
		// This may not be considered an error if the caller is doing a partial
		// decode.
		return ErrResourceExhausted
	}

	return nil
}

func (d *decoder) verifyFrameCheckSequence() bool {
	// De-ring the last four bytes read, which at this point contain the FCS.
	fcsBuffer := make([]byte, 4)
	index := d.lastReadByteIndex

	for i := 0; i < len(fcsBuffer); i++ {
		fcsBuffer[i] = d.lastReadBytes[index]
		index = (index + 1) % len(d.lastReadBytes)
	}

	fcs := binary.LittleEndian.Uint32(fcsBuffer)

	return fcs == d.fcs
}
