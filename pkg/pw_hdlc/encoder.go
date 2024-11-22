package pw_hdlc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"

	"github.com/robertfarnum/go_pw_rpc/pkg/pw_varint"
)

type Encoder struct {
	writer  io.Writer
	address uint64
	data    []byte
	fcs     uint32
}

func NewEncoder(writer io.Writer, address uint64) *Encoder {
	return &Encoder{
		writer:  writer,
		address: address,
		data:    make([]byte, 0),
		fcs:     0,
	}
}

func (e *Encoder) fillPayload(payload []byte) {
	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, payload)
	payload = bytes.Replace(payload, []byte{kFlag}, kEscapedFlag, -1)
	payload = bytes.Replace(payload, []byte{kEscape}, kEscapedEscape, -1)
	e.data = append(e.data, payload...)
}

func (e *Encoder) finishFrame() {
	fcsBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(fcsBytes, e.fcs)
	e.data = append(e.data, fcsBytes...)
	e.data = append(e.data, kFlag)
}

func (e *Encoder) startFrame(address uint64, control byte) error {
	e.data = append(e.data, kFlag)

	addressBytes, err := pw_varint.Encode(address, 10, pw_varint.OneTerminatedLeastSignificant)
	if err != nil {
		return err
	}

	e.data = append(e.data, addressBytes...)
	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, addressBytes)
	e.data = append(e.data, control)
	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, []byte{control})

	return nil
}

func (e *Encoder) WritePayload(payload []byte) (int, error) {
	err := e.startFrame(e.address, kUnnumberedUFrame)
	if err != nil {
		return 0, err
	}

	e.fillPayload(payload)

	e.finishFrame()

	return e.writer.Write(e.data)
}
