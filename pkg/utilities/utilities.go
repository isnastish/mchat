package util

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"strings"
	"time"
)

func Sha256Checksum(bytes []byte) string {
	sum := sha256.Sum256(bytes)
	return fmt.Sprintf("%X", sum)
}

func EndOfLine(src string) string {
	return src + "\r\n"
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

func TimeNowStr() string {
	return time.Now().Format(time.DateTime)
}

func Fmt(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func TrimWhitespaces(src []byte) []byte {
	return []byte(strings.Trim(string(src), " \r\n\v\t\f"))
}
