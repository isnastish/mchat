// TODO: Test storing messages with non-existent channel
package redis

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/isnastish/chat/pkg/testsetup"

	"github.com/isnastish/chat/pkg/backend"
)

func TestMain(m *testing.M) {
	var result int
	var redisHasStarted bool

	if os.Getenv("CI") == "" {
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
}

var redisConfig = backend.RedisConfig{
	Endpoint: "127.0.0.1:6379",
	Password: "",
	Username: "",
}

func clearChannels(rb *redisBackend, t *testing.T) {
	for _, ch := range testsetup.Channels {
		rb.DeleteChannel(ch.Name)
		assert.False(t, rb.HasChannel(ch.Name))
	}
}

func clearParticipants(rb *redisBackend, t *testing.T) {
	for _, p := range testsetup.Participants {
		rb.deleteParticipant(p.Username)
		assert.False(t, rb.HasParticipant(p.Username))
	}
}

func TestRegisterParticipant(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearParticipants(backend, t) // clear the state
	defer clearParticipants(backend, t)
	for _, p := range testsetup.Participants {
		backend.RegisterParticipant(&p)
		assert.True(t, backend.HasParticipant(p.Username))
	}
	participants := backend.GetParticipants()
	assert.True(t, testsetup.Match(participants, testsetup.Participants, testsetup.ContainsParticipant))
}

func TestParticipantAlreadyExists(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearParticipants(backend, t) // clear the state
	defer clearParticipants(backend, t)
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	assert.Panics(t, func() { backend.RegisterParticipant(&testsetup.Participants[0]) })
}

func TestRegisAuthenticateParticipant(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearParticipants(backend, t)
	defer clearParticipants(backend, t)
	backend.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, backend.HasParticipant(testsetup.Participants[0].Username))
	assert.True(t, backend.AuthParticipant(&testsetup.Participants[0]))
}

func TestRegisterChannel(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearChannels(backend, t)
	defer clearChannels(backend, t)
	for _, ch := range testsetup.Channels {
		backend.RegisterChannel(&ch)
		assert.True(t, backend.HasChannel(ch.Name))
	}
	channels := backend.GetChannels()
	assert.True(t, testsetup.Match(channels, testsetup.Channels, testsetup.ContainsChannel))
}

func TestChannelAlreadyExists(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearChannels(backend, t)
	defer clearChannels(backend, t)
	defer backend.DeleteChannel(testsetup.Channels[0].Name)
	backend.RegisterChannel(&testsetup.Channels[0])
	assert.True(t, backend.HasChannel(testsetup.Channels[0].Name))
	assert.Panics(t, func() { backend.RegisterChannel(&testsetup.Channels[0]) })
}

func TestStoreGeneralMessages(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	defer func() {
		backend.deleteMessages()
		assert.Equal(t, len(backend.GetChatHistory()), 0)
	}()

	for _, msg := range testsetup.GeneralMessages {
		backend.StoreMessage(&msg)
	}

	chatHistory := backend.GetChatHistory()
	assert.True(t, testsetup.Match(chatHistory, testsetup.GeneralMessages, testsetup.ContainsMessage))
}

func TestStoreChannelMessages(t *testing.T) {
	backend, err := NewRedisBackend(&redisConfig)
	assert.True(t, err == nil)
	clearChannels(backend, t)
	defer clearChannels(backend, t)
	for _, ch := range testsetup.Channels {
		backend.RegisterChannel(&ch)
		assert.True(t, backend.HasChannel(ch.Name))
	}

	defer func() {
		backend.deleteMessages(testsetup.Channels[0].Name)
		assert.Equal(t, len(backend.GetChatHistory(testsetup.Channels[0].Name)), 0)
	}()
	for _, msg := range testsetup.BooksChannelMessages {
		backend.StoreMessage(&msg)
	}

	channelHistory := backend.GetChatHistory(testsetup.Channels[0].Name)
	assert.True(t, testsetup.Match(channelHistory, testsetup.BooksChannelMessages, testsetup.ContainsMessage))
}
