// NOTE: For now it contains some dummy tests which should be removed.
package client

// import (
// 	"bufio"
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"io"
// 	"net"
// 	"os"
// 	"testing"
// 	"time"

// 	"go.uber.org/goleak"

// 	"github.com/isnastish/chat/pkg/utilities"
// )

// func TestConnectedSuccessfully(t *testing.T) {
// 	defer goleak.VerifyNone(t)

// 	client := CreateClient(&Config{
// 		Network:      "tcp",
// 		Addr:         "127.0.0.1:5000",
// 		RetriesCount: 5,
// 	})
// 	client.Run()
// }

// func TestDoesBufioReadBlock(t *testing.T) {
// 	defer goleak.VerifyNone(t)
// 	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
// 	defer cancel()

// 	go func() {
// 		reader := bufio.NewReader(os.Stdin)
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				fmt.Println("context was canceled")
// 				return
// 			default:
// 				buffer := bytes.NewBuffer(make([]byte, 1024))
// 				n, err := io.Copy(buffer, reader)
// 				if err != nil && err != io.EOF {
// 					fmt.Println("Error occured")
// 					return
// 				}

// 				if n == 0 {
// 					// return
// 					fmt.Println("zero bytes read")
// 				}

// 				fmt.Printf("Reading input: %d\n", buffer.Len())
// 			}
// 		}
// 	}()
// 	<-time.After(2 * time.Second)
// }

// func TestClosingConnectionOnOppositeSideCausesZeroBytesRead(t *testing.T) {
// 	defer goleak.VerifyNone(t)

// 	go func() {
// 		for {
// 			remoteConn, err := net.Dial("tcp", "127.0.0.1:5000")
// 			if err != nil {
// 				time.Sleep(1 * time.Second)
// 				continue
// 			}

// 			for {
// 				tmpBuf := make([]byte, 1024)
// 				bytesRead, err := remoteConn.Read(tmpBuf)
// 				trimmedBuffer := util.TrimWhitespaces(tmpBuf[:bytesRead])
// 				buffer := bytes.NewBuffer(trimmedBuffer)
// 				if err != nil && err != io.EOF {
// 					fmt.Println("error", err)
// 					return
// 				}

// 				if bytesRead == 0 {
// 					fmt.Println("An opposide side closed the connection")
// 					return
// 				}

// 				_ = buffer
// 			}
// 		}
// 	}()

// 	l, err := net.Listen("tcp", "127.0.0.1:5000")
// 	if err != nil {
// 		return
// 	}

// 	for {
// 		conn, err := l.Accept()
// 		if err == nil {
// 			<-time.After(3 * time.Second)
// 			conn.Close()
// 		}
// 		break
// 	}
// }
