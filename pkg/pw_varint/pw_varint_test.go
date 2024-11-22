package pw_varint

import "testing"

func TestDecode(t *testing.T) {
	_, value := Decode([]byte{0x01, 0x10}, ZeroTerminatedLeastSignificant)
	if value != 1024 {
		t.Fatalf("%d != %d", value, 1024)
	}
}
