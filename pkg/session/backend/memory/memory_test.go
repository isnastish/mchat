package memory

import (
	"testing"

	"github.com/isnastish/chat/pkg/session/backend"
	"github.com/stretchr/testify/assert"
)

var plist = []backend.Participant{
	{
		Name:           "nasayer1234",
		PasswordSha256: "nasayerLive234@Outline",
		EmailAddress:   "nasayer@gmail.com",
	},
	{
		Name:           "anonymus__",
		PasswordSha256: "SomeRandomPass8ordHere",
		EmailAddress:   "anonymus@mail.ru",
	},
}

var pchannels = []backend.Channel{
	{
		Name:  "BooksChannel",
		Desc:  "Channel to shared the books",
		Owner: "SarahObrian",
	},
	{
		Name:  "ProgrammingChannel",
		Desc:  "Channel for sharing programming related things",
		Owner: "Anna",
	},
}

var pmessages = []backend.ParticipantMessage{
	{
		Contents: []byte("This is the message contents."),
		Sender:   "Elizabeth",
	},
	{
		Contents: []byte("Hello! My name is Anna."),
		Sender:   "Anna",
	},
	{
		Contents: []byte("Hi! How are you doing Anna?"),
		Sender:   "Elizabeth",
	},
	{
		Contents: []byte("Hi everyone, I was reading an interesting novel relatively recently and want to share some thought with you."),
		Sender:   "Herman",
		Channel:  "books_channel",
	},
}

func registerParticipant(storage *MemoryBackend, p *backend.Participant) {
	storage.RegisterParticipant(p.Name, p.PasswordSha256, p.EmailAddress)
}

func registerChannel(storage *MemoryBackend, c *backend.Channel) {
	storage.RegisterChannel(c.Name, c.Desc, c.Owner)
}

func storeMessage(storage *MemoryBackend, message *backend.ParticipantMessage) {
	if message.Channel != "" {
		storage.StoreMessage(message.Sender, message.Time, message.Contents, message.Channel)
		return
	}
	storage.StoreMessage(message.Sender, message.Time, message.Contents)
}

func TestRegisterParticipant(t *testing.T) {
	storage := NewMemoryBackend()
	for _, p := range plist {
		registerParticipant(storage, &p)
		assert.True(t, storage.HasParticipant(p.Name))
	}
	// participant already exists
	assert.Panics(t, func() {
		registerParticipant(storage, &plist[0])
	})
	participants := storage.GetParticipantList()
	assert.Equal(t, len(plist), len(participants))
}

func TestAuthnticateParticipant(t *testing.T) {
	storage := NewMemoryBackend()
	registerParticipant(storage, &plist[0])
	assert.True(t, storage.HasParticipant(plist[0].Name))
	assert.True(t, storage.AuthParticipant(plist[0].Name, plist[0].PasswordSha256))
}

func TestChannelCreationDeletion(t *testing.T) {
	storage := NewMemoryBackend()
	for _, ch := range pchannels {
		registerChannel(storage, &ch)
		assert.True(t, storage.HasChannel(ch.Name))
	}
	channels := storage.GetChannels()
	assert.Equal(t, len(pchannels), len(channels))
	// delete channel
	storage.DeleteChannel(pchannels[0].Name)
	assert.False(t, storage.HasChannel(pchannels[0].Name))
}

func TestChannelAlreadyExists(t *testing.T) {
	storage := NewMemoryBackend()
	registerChannel(storage, &pchannels[0])
	assert.True(t, storage.HasChannel(pchannels[0].Name))
	assert.Panics(t, func() { registerChannel(storage, &pchannels[0]) })
}

func TestStoreMessage(t *testing.T) {
	storage := NewMemoryBackend()
	for _, msg := range pmessages[0:3] {
		storeMessage(storage, &msg)
	}
	history := storage.GetChatHistory()
	assert.Equal(t, len(history), len(pmessages[0:3]))
	for index, msg := range history {
		assert.Equal(t, msg.Contents, pmessages[index].Contents)
	}
	// store message as a part of channels history
}
