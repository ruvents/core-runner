package runner

import (
	"crypto/rand"
	"fmt"
)

type UUID4 string

// https://www.rfc-editor.org/rfc/rfc4122
func NewUUID4() UUID4 {
	buf := make([]byte, 16)
	rand.Read(buf)
	buf[6] = 0x40 | (buf[6] & 0x0f)
	buf[8] = 0x80 | (buf[8] & 0xf)
	return UUID4(fmt.Sprintf(
		"%x-%x-%x-%x-%x",
		buf[:4], buf[4:6], buf[6:8], buf[8:10], buf[10:],
	))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
