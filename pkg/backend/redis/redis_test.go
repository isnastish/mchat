package redis

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/isnastish/chat/pkg/testsetup"
)

func TestMain(m *testing.M) {
	var result int
	var redisHasStarted bool

	defer func() {
		if redisHasStarted {
			testsetup.TeardownRedisMock()
		}
		os.Exit(result)
	}()

	redisHasStarted, _ = testsetup.SetupRedisMock()
	result = m.Run()
}

func TestRegisterParticipant(t *testing.T) {
	rb, err := NewRedisBackend("127.0.0.1:6379")
	assert.True(t, err == nil)
	_ = rb
}
