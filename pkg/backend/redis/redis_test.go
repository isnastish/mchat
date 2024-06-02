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

	redisHasStarted, _ = testsetup.SetupRedisMock()
	result = m.Run()

	defer func() {
		if redisHasStarted {
			testsetup.TeardownRedisMock()
		}
		os.Exit(result)
	}()
}

func TestRegisterParticipant(t *testing.T) {
	backend, err := NewRedisBackend("127.0.0.1:6379")
	assert.True(t, err == nil)
	for _, p := range testsetup.Participants {
		backend.RegisterParticipant(&p)
		assert.True(t, backend.HasParticipant(p.Username))
	}
}
