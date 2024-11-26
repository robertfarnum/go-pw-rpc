package pw_hdlc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"sync"

	"github.com/robertfarnum/go-pw-rpc/pkg/pw_varint"
)

type Encoder interface {
	Encode(payload []byte) error
}

func NewEncoder(writer io.Writer, address uint64) Encoder {
	return &encoder{
		writer:  writer,
		address: address,
		fcs:     0,
	}
}

type encoder struct {
	writer  io.Writer
	address uint64
	fcs     uint32
	mu      sync.Mutex
}

func (e *encoder) Encode(payload []byte) error {
	e.mu.Lock()

	frame := e.startFrame(e.address, kUnnumberedUFrame)

	frame = append(frame, e.getPayload(payload)...)

	frame = append(frame, e.finishFrame()...)

	_, err := e.writer.Write(frame)
	if err != nil {
		e.mu.Unlock()
		return err
	}

	e.mu.Unlock()
	return nil
}

func (e *encoder) getPayload(payload []byte) []byte {
	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, payload)

	payload = bytes.Replace(payload, []byte{kFlag}, kEscapedFlag, -1)
	payload = bytes.Replace(payload, []byte{kEscape}, kEscapedEscape, -1)

	return payload
}

func (e *encoder) finishFrame() []byte {
	finishFrame := make([]byte, 4)

	binary.LittleEndian.PutUint32(finishFrame, e.fcs)
	e.fcs = 0

	finishFrame = append(finishFrame, kFlag)

	return finishFrame
}

func (e *encoder) startFrame(address uint64, control byte) []byte {
	startFrame := make([]byte, 0)

	startFrame = append(startFrame, kFlag)

	addressBytes := pw_varint.Encode(address, pw_varint.OneTerminatedLeastSignificant)

	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, addressBytes)
	startFrame = append(startFrame, addressBytes...)

	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, []byte{control})
	startFrame = append(startFrame, control)

	return startFrame
}
