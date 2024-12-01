package pw_hdlc

type Frame struct {
	address uint64
	control byte
	payload []byte
}

func NewFrame(address uint64, control byte, payload []byte) *Frame {
	return &Frame{
		address: address,
		control: control,
		payload: payload,
	}
}

func (f *Frame) Payload() []byte {
	return f.payload
}

func (f *Frame) Address() uint64 {
	return f.address
}

func (f *Frame) Control() byte {
	return f.control
}
