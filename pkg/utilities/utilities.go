package util

import (
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

func endOfLine(src string) string {
	return src + "\r\n"
}

func TimeNowStr() string {
	return time.Now().Format(time.TimeOnly)
}

func Fmt(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func Fmtln(format string, args ...any) string {
	return fmt.Sprintf(endOfLine(format), args...)
}

func TrimWhitespaces(src []byte) []byte {
	return []byte(strings.Trim(string(src), " \r\n\v\t\f"))
}

func Sleep(duration int64) {
	<-time.After(time.Duration(duration) * time.Millisecond)
}

func WriteBytesToConn(conn net.Conn, bytes []byte, size int) (int, error) {
	if size < 0 {
		panic("Cannot write negative number of bytes")
	}
	var bytesWritten int
	for bytesWritten < size {
		thisWrite, err := conn.Write(bytes)
		if err != nil {
			return bytesWritten, err
		}
		bytesWritten += thisWrite
	}
	return bytesWritten, nil
}
