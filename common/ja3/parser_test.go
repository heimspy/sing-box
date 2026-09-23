package ja3

import (
	"encoding/binary"
	"slices"
	"testing"
)

func TestALPN(t *testing.T) {
	for _, test := range []struct {
		name    string
		data    []byte
		want    []string
		invalid bool
	}{
		{name: "HTTP3", data: []byte{0, 3, 2, 'h', '3'}, want: []string{"h3"}},
		{name: "multiple", data: []byte{0, 6, 2, 'h', '3', 2, 'h', '2'}, want: []string{"h3", "h2"}},
		{name: "other QUIC", data: []byte{0, 4, 3, 'f', 'o', 'o'}, want: []string{"foo"}},
		{name: "truncated vector", data: []byte{0, 9, 2, 'h', '3'}, invalid: true},
		{name: "truncated protocol", data: []byte{0, 3, 5, 'h', '3'}, invalid: true},
		{name: "empty protocol", data: []byte{0, 1, 0}, invalid: true},
		{name: "empty list", data: []byte{0, 0}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			extension := binary.BigEndian.AppendUint16(nil, uint16(4+len(test.data)))
			extension = binary.BigEndian.AppendUint16(extension, 16)
			extension = binary.BigEndian.AppendUint16(extension, uint16(len(test.data)))
			extension = append(extension, test.data...)
			var hello ClientHello
			err := hello.parseExtensions(extension)
			if (err != nil) != test.invalid {
				t.Fatalf("unexpected parse result: %v", err)
			}
			if !test.invalid && !slices.Equal(hello.ALPN, test.want) {
				t.Fatalf("protocols=%v want=%v", hello.ALPN, test.want)
			}
		})
	}
}
