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

	// Make sure we always tear down the running redis-mock container
	// if one of the tests panics.
	defer func() {
		if redisHasStarted {
			testsetup.TeardownRedisMock()
		}
		os.Exit(result)
	}()
}

var redisEndpoint = "127.0.0.1:6379"

func TestRegisterParticipant(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	defer func() {
		// Make sure all the participants are deleted so it does't affect the subsequent tests.
		for _, p := range testsetup.Participants {
			backend.DeleteParticipant(&p)
			assert.False(t, backend.HasParticipant(p.Username))
		}
	}()
	for _, p := range testsetup.Participants {
		backend.RegisterParticipant(&p)
		assert.True(t, backend.HasParticipant(p.Username))
	}
}

func TestParticipantAlreadyExists(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	defer backend.DeleteParticipant(&testsetup.Participants[0])
	assert.Panics(t, func() { backend.RegisterParticipant(&testsetup.Participants[0]) })
}

func TestRegisAuthenticateParticipant(t *testing.T) {
	backend, err := NewRedisBackend(redisEndpoint)
	assert.True(t, err == nil)
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	defer backend.DeleteParticipant(&testsetup.Participants[0])
	assert.True(t, backend.AuthParticipant(&testsetup.Participants[0]))
}
