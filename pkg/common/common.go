package common

func StripCR(buf []byte, bytesRead int) []byte {
	for back := bytesRead - 1; back >= 0; back-- {
		if buf[back] == '\n' || buf[back] == '\r' {
			bytesRead--
			continue
		}
		break
	}
	return buf[:bytesRead]
}
