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

func NewBackend() *MemoryBackend {
	return &MemoryBackend{
		participants:       make(map[string]*backend.Participant),
		channels:           make(map[string]*backend.Channel),
		generalChatHistory: make([]*backend.ParticipantMessage, 0, 1024),
	}
}

func (b *MemoryBackend) HasParticipant(username string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, exists := b.participants[username]
	return exists
}

func (b *MemoryBackend) RegisterParticipant(username string, passwordShaw256 string, emailAddress string) {
	b.mu.Lock()
	_, exists := b.participants[username]
	if exists {
		panic("participant already exists")
	}

	b.participants[username] = &backend.Participant{
		Name:           username,
		PasswordSha256: passwordShaw256,
		EmailAddress:   emailAddress,
		JoinTime:       time.Now().Format(time.DateTime),
	}
	b.mu.Unlock()
}

func (b *MemoryBackend) AuthParticipant(username string, passwordSha256 string) bool {
	b.mu.Lock()
	defer b.mu.Lock()
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
	_, exists := b.channels[channelName]
	return exists
}

func (b *MemoryBackend) RegisterChannel(channelName string, desc string, ownerName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.HasChannel(channelName) {
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
	return b.HasChannel(channelName)
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
	channels := make([]*backend.Channel, len(b.channels))
	for _, ch := range b.channels {
		channels = append(channels, ch)
	}
	return channels
}

func (b *MemoryBackend) GetParticipantList() []*backend.Participant {
	b.mu.Lock()
	defer b.mu.Unlock()

	participantList := make([]*backend.Participant, len(b.participants))
	for _, participant := range b.participants {
		participantList = append(participantList, participant)
	}

	return participantList
}
