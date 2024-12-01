package pw_hdlc

import (
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
	defer e.mu.Unlock()

	err := e.startFrame(e.address, kUnnumberedUFrame)
	if err != nil {
		return err
	}

	err = e.writeData(payload)
	if err != nil {
		return err
	}

	return e.finishFrame()
}

func (e *encoder) escapeAndWrite(b byte) error {
	if b == kFlag {
		_, err := e.writer.Write(kEscapedFlag)
		return err
	}
	if b == kEscape {
		_, err := e.writer.Write(kEscapedEscape)
		return err
	}
	_, err := e.writer.Write([]byte{b})

	return err
}

func (e *encoder) writeData(data []byte) error {
	for _, b := range data {
		err := e.escapeAndWrite(b)
		if err != nil {
			return err
		}
	}

	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, data)

	return nil
}

func (e *encoder) finishFrame() error {
	fcsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(fcsBytes, e.fcs)
	err := e.writeData(fcsBytes)
	if err != nil {
		return err
	}

	_, err = e.writer.Write([]byte{kFlag})

	return err
}

func (e *encoder) startFrame(address uint64, control byte) error {
	e.fcs = 0

	_, err := e.writer.Write([]byte{kFlag})
	if err != nil {
		return err
	}

	metadataBytes := pw_varint.Encode(address, pw_varint.OneTerminatedLeastSignificant)
	if len(metadataBytes) == 0 {
		return ErrShortBuffer
	}

	metadataBytes = append(metadataBytes, control)

	return e.writeData(metadataBytes)
}
