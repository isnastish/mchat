package util

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"os/exec"
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

func ClearScreen() {
	// TODO: Once the user entered all the data, the screen has to be cleared on its side.
	// So we have enough space for dispalying messages.
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func Sleep(duration int64) {
	<-time.After(time.Duration(duration) * time.Millisecond)
}
