package utilities

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"strings"
)

func Sha256Checksum(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return fmt.Sprintf("%X", sum)
}

func Cr(builder *strings.Builder) {
	builder.WriteString("\r\n")
}

func WriteBytes(conn net.Conn, buffer *bytes.Buffer) (int, error) {
	var bWritten int
	for bWritten < buffer.Len() {
		n, err := conn.Write(buffer.Bytes())
		if err != nil {
			return bWritten, err
		}
		bWritten += n
	}
	return bWritten, nil
}

// func Message(msgstr string) *bytes.Buffer {
// 	msgsize := len(msgstr)
// 	buf := bytes.NewBuffer(make([]byte, msgsize+2)) // for \r\n
// 	buf.WriteString(msgstr)
// 	return buf
// }
