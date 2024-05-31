package redis

import (
	_ "github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	var result int
	var redisHasStarted bool
	defer func() {
		if redisHasStarted {
			teardownRedisMock()
		}
		os.Exit(result)
	}()

	redisHasStarted, _ = setupRedisMock()
	if redisHasStarted {
		teardownRedisMock()
	}

	result = m.Run()
}

func TestRegisterParticipant(t *testing.T) {
	rb, err := NewRedisBackend("127.0.0.1:6379")
	if err != nil {
		return
	}

	_ = rb
}
