package memory

import (
	"fmt"
	"strings"
	"sync"
	"time"

	backend "github.com/isnastish/chat/pkg/session/backend"
)

type MemoryBackend struct {
	participants       map[string]*backend.Participant
	channels           map[string]*backend.Channel
	generalChatHistory []*backend.ParticipantMessage
	mu                 sync.Mutex
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		participants:       make(map[string]*backend.Participant),
		channels:           make(map[string]*backend.Channel),
		generalChatHistory: make([]*backend.ParticipantMessage, 0, 1024),
	}
}

func (b *MemoryBackend) doesParticipantExist(username string) bool {
	_, exists := b.participants[username]
	return exists
}

func (b *MemoryBackend) doesChannelExist(channelName string) bool {
	_, exists := b.channels[channelName]
	return exists
}

func (b *MemoryBackend) HasParticipant(username string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.doesParticipantExist(username)
}

func (b *MemoryBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {
	// TODO(alx): Do the password sha256 validation here instead of session.
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.doesParticipantExist(username) {
		panic(fmt.Sprintf("participant {%s} already exists", username))
	}

	b.participants[username] = &backend.Participant{
		Name:           username,
		PasswordSha256: passwordShaw256,
		EmailAddress:   emailAddress,
		JoinTime:       time.Now().Format(time.DateTime),
	}
}

func (b *MemoryBackend) AuthParticipant(username string, passwordSha256 string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	participant, exists := b.participants[username]
	if exists {
		return strings.EqualFold(participant.PasswordSha256, passwordSha256)
	}
	return false
}

func (b *MemoryBackend) StoreMessage(senderName string, sentTime string, contents []byte, channelName ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	message := &backend.ParticipantMessage{
		Sender:   senderName,
		Time:     sentTime,
		Contents: contents,
	}

	if len(channelName) > 0 {
		channel, exists := b.channels[channelName[0]]
		if !exists {
			panic(fmt.Sprintf("cannot store a mesasge, channel {%s} doesn't exist", channelName[0]))
		}
		message.Channel = channelName[0]
		channel.ChatHistory = append(channel.ChatHistory, message)
	} else {
		b.generalChatHistory = append(b.generalChatHistory, message)
	}
}

func (b *MemoryBackend) HasChannel(channelName string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.doesChannelExist(channelName)
}

func (b *MemoryBackend) RegisterChannel(channelName string, desc string, ownerName string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.doesChannelExist(channelName) {
		panic(fmt.Sprintf("channel {%s} already exists", channelName))
	}

	b.channels[channelName] = &backend.Channel{
		Name:         channelName,
		Desc:         desc,
		CreationDate: time.Now().Format(time.DateTime),
		ChatHistory:  make([]*backend.ParticipantMessage, 0, 1024),
		Participants: make([]*backend.Participant, 0),
	}
}

func (b *MemoryBackend) DeleteChannel(channelName string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.channels, channelName)
	return b.doesChannelExist(channelName)
}

func (b *MemoryBackend) GetChatHistory(channelName ...string) []*backend.ParticipantMessage {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(channelName) > 0 {
		channel, exists := b.channels[channelName[0]]
		if !exists {
			panic(fmt.Sprintf("channel {%s} doesn't exist", channelName[0]))
		}
		return channel.ChatHistory
	}
	return b.generalChatHistory
}

func (b *MemoryBackend) GetChannels() []*backend.Channel {
	b.mu.Lock()
	defer b.mu.Unlock()

	cap := len(b.channels)
	channels := make([]*backend.Channel, 0, cap)
	for _, ch := range b.channels {
		channels = append(channels, ch)
	}

	return channels
}

func (b *MemoryBackend) GetParticipantList() []*backend.Participant {
	b.mu.Lock()
	defer b.mu.Unlock()

	cap := len(b.participants)
	plist := make([]*backend.Participant, 0, cap)
	for _, participant := range b.participants {
		plist = append(plist, participant)
	}

	return plist
}
