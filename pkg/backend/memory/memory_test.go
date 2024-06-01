package memory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/isnastish/chat/pkg/testsetup"
)

func TestRegisterParticipant(t *testing.T) {
	storage := NewMemoryBackend()
	for _, p := range testsetup.Participants {
		storage.RegisterParticipant(&p)
		assert.True(t, storage.HasParticipant(p.Username))
	}
	assert.Panics(t, func() { storage.RegisterParticipant(&testsetup.Participants[0]) })
	partList := storage.GetParticipantList()
	assert.Equal(t, len(partList), len(testsetup.Participants))
}

func TestAuthnticateParticipant(t *testing.T) {
	storage := NewMemoryBackend()
	storage.RegisterParticipant(&testsetup.Participants[0])
	assert.True(t, storage.HasParticipant(testsetup.Participants[0].Username))
	assert.True(t, storage.AuthParticipant(&testsetup.Participants[0]))
}

func TestChannelCreationDeletion(t *testing.T) {
	storage := NewMemoryBackend()
	for _, ch := range testsetup.Channels {
		storage.RegisterChannel(&ch)
		assert.True(t, storage.HasChannel(ch.Name))
	}
	channels := storage.GetChannels()
	assert.Equal(t, len(testsetup.Channels), len(channels))
	// delete channel
	assert.True(t, storage.DeleteChannel(testsetup.Channels[0].Name))
	// delete already deleted channel
	assert.Panics(t, func() { storage.DeleteChannel(testsetup.Channels[0].Name) })
}

func TestChannelAlreadyExists(t *testing.T) {
	storage := NewMemoryBackend()
	storage.RegisterChannel(&testsetup.Channels[0])
	assert.True(t, storage.HasChannel(testsetup.Channels[0].Name))
	assert.Panics(t, func() { storage.RegisterChannel(&testsetup.Channels[0]) })
}

func TestStoreMessage(t *testing.T) {
	storage := NewMemoryBackend()
	for _, msg := range testsetup.GeneralMessages {
		storage.StoreMessage(&msg)
	}
	history := storage.GetChatHistory()
	assert.Equal(t, len(history), len(testsetup.GeneralMessages))
	for index, msg := range history {
		assert.Equal(t, msg.Contents, testsetup.GeneralMessages[index].Contents)
	}
}

func TestGetChannelHistory(t *testing.T) {

}
