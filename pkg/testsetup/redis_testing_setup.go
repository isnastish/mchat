// NOTE: Initialy this file was a part of the redis package, but since it would have been included into the package build,
// I had to move it out into a separate package `testsetup`. All the files with _test.go suffix are excluded form package build.

package testsetup

import (
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/isnastish/chat/pkg/logger"
)

var redisContainerName = "redis-mock"
var redisImage = "redis:7.2.5"

func SetupRedisMock() (bool, error) {
	hasStarted := false

	cmd := exec.Command("docker", "run", "--rm", "--name", redisContainerName, "-p", "6379:6379", redisImage)
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
			log.Logger.Fatal("Reading stdout failed: %v", err)
		}
		if n > 0 {
			stdoutStr := string(buf[:n])
			log.Logger.Info(stdoutStr)
			strBuilder.WriteString(stdoutStr)
			if strings.Contains(strBuilder.String(), "Ready to accept connections") {
				break
			}
		}
	}
	return hasStarted, err
}

func TeardownRedisMock() {
	cmd := exec.Command("docker", "rm", "-f", redisContainerName)
	err := cmd.Run()
	if err != nil {
		log.Logger.Error("Failed to tread down redis mock container: %s", err)
	} else {
		log.Logger.Info("Redis mock container shut down")
	}
}
