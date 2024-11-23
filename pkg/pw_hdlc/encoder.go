package pw_hdlc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"

	"github.com/robertfarnum/go_pw_rpc/pkg/pw_varint"
)

type Encoder struct {
	address uint64
	fcs     uint32
}

func NewEncoder(address uint64) *Encoder {
	return &Encoder{
		address: address,
		fcs:     0,
	}
}

func (e *Encoder) getPayload(payload []byte) []byte {
	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, payload)

	payload = bytes.Replace(payload, []byte{kFlag}, kEscapedFlag, -1)
	payload = bytes.Replace(payload, []byte{kEscape}, kEscapedEscape, -1)

	return payload
}

func (e *Encoder) finishFrame() []byte {
	finishFrame := make([]byte, 4)

	binary.LittleEndian.PutUint32(finishFrame, e.fcs)
	finishFrame = append(finishFrame, kFlag)

	return finishFrame
}

func (e *Encoder) startFrame(address uint64, control byte) []byte {
	startFrame := make([]byte, 0)

	startFrame = append(startFrame, kFlag)

	addressBytes := pw_varint.Encode(address, pw_varint.OneTerminatedLeastSignificant)

	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, addressBytes)
	startFrame = append(startFrame, addressBytes...)

	e.fcs = crc32.Update(e.fcs, crc32.IEEETable, []byte{control})
	startFrame = append(startFrame, control)

	return startFrame
}

func (e *Encoder) Encode(payload []byte) []byte {
	frame := e.startFrame(e.address, kUnnumberedUFrame)

	frame = append(frame, e.getPayload(payload)...)

	frame = append(frame, e.finishFrame()...)

	return frame
}
