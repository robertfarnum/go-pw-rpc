package pw_hdlc

import (
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/robertfarnum/go_pw_rpc/pkg/pw_varint"
)

type Decoder struct {
	reader            io.Reader
	address           uint64
	buffer            []byte
	state             State
	currentFrameSize  int
	lastReadBytes     []byte
	lastReadByteIndex int
	fcs               uint32
}

func NewDecoder(reader io.Reader, address uint64) *Decoder {
	return &Decoder{
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

func (d *Decoder) parse(frame []byte) (*Frame, error) {
	address, addressSize := pw_varint.Decode(frame, pw_varint.OneTerminatedLeastSignificant)

	dataSize := len(frame) - addressSize - kControlSize - kFcsSize

	if addressSize < 1 || dataSize < 0 {
		return nil, ErrDataLoss
	}

	start := addressSize + 1
	end := start + dataSize
	return NewFrame(address, frame[addressSize], frame[start:end]), nil
}

func (d *Decoder) Process() (*Frame, error) {
	buf := make([]byte, 1)

	for {
		_, err := d.reader.Read(buf)
		if err != nil {
			return nil, ErrDataLoss
		}
		frame, err := d.process(buf[0])
		if err == ErrUnavailable {
			continue
		} else if err != nil {
			return nil, err
		}

		return frame, err
	}
}

func (d *Decoder) reset() {
	d.currentFrameSize = 0
	d.lastReadByteIndex = 0
	d.fcs = 0
	d.state = kInterFrame
	d.buffer = d.buffer[:0]
}

func (d *Decoder) escape(b byte) byte {
	return b ^ kEscapeConstant
}

func (d *Decoder) process(newByte byte) (*Frame, error) {
	switch d.state {
	case kInterFrame:
		if newByte == kFlag {
			d.state = kFrame

			// Report an error if non-flag bytes were read between frames.
			if d.currentFrameSize != 0 {
				d.reset()
				return nil, ErrDataLoss
			}
		} else {
			// Count bytes to track how many are discarded.
			d.currentFrameSize += 1
		}
		return nil, ErrUnavailable // Report error when starting a new frame.
	case kFrame:
		if newByte == kFlag {
			err := d.checkFrame()
			if err == nil {
				return d.parse(d.buffer[0:d.currentFrameSize])
			}

			d.reset()

			return nil, err
		}

		if newByte == kEscape {
			d.state = kFrameEscape
		} else {
			d.appendByte(newByte)
		}
		return nil, ErrUnavailable
	case kFrameEscape:
		// The flag character cannot be escaped; return an error.
		if newByte == kFlag {
			d.state = kFrame
			d.reset()
			return nil, ErrDataLoss
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
		return nil, ErrUnavailable
	}

	return nil, ErrDataLoss
}

func (d *Decoder) appendByte(newByte byte) {
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

func (d *Decoder) checkFrame() error {
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

func (d *Decoder) verifyFrameCheckSequence() bool {
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
