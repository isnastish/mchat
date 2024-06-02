package utilities

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func Sha256Checksum(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return fmt.Sprintf("%X", sum)
}

func Cr(builder *strings.Builder) {
	builder.WriteString("\r\n")
}

// func Message(msgstr string) *bytes.Buffer {
// 	msgsize := len(msgstr)
// 	buf := bytes.NewBuffer(make([]byte, msgsize+2)) // for \r\n
// 	buf.WriteString(msgstr)
// 	return buf
// }
