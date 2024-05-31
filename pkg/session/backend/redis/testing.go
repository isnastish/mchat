package redis

import (
	_ "context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

func setupRedisMock() (bool, error) {
	hasStarted := false

	cmd := exec.Command("docker", "run", "--rm", "--name", "redis-mock", "-p", "6379:6379", "redis")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return hasStarted, err
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		return hasStarted, err
	}

	hasStarted = true

	var strBuilder strings.Builder
	// give a container some time to spin up. Only pulling the image might take a couple of minutes.
	timer := time.NewTimer(3 * time.Minute)
	buf := make([]byte, 256)
	for {
		select {
		case <-timer.C:
			return hasStarted, err
		default:
		}
		n, err := stdout.Read(buf[:])
		if err != nil {
			if err == io.EOF {
				break
			}
		}
		if n > 0 {
			stdoutStr := string(buf[:n])
			fmt.Println(stdoutStr)
			strBuilder.WriteString(stdoutStr)
			if strings.Contains(strBuilder.String(), "Ready to accept connections") {
				break
			}
		}
	}
	return hasStarted, err
}

func teardownRedisMock() {
	cmd := exec.Command("docker", "rm", "-f", "redis-mock")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("failed to tear down the container: %s\n", err.Error())
	} else {
		fmt.Printf("redis-emulator was shut down")
	}
}
