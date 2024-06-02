package memory

import (
	"bytes"
	"strings"
	"sync"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type MemoryBackend struct {
	participants map[string]*types.Participant
	chatHistory  []*types.ChatMessage
	channels     map[string]*types.Channel
	mu           sync.Mutex
}

func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		participants: make(map[string]*types.Participant),
		chatHistory:  make([]*types.ChatMessage, 0, 1024),
		channels:     make(map[string]*types.Channel),
	}
}

func (m *MemoryBackend) doesParticipantExist(username string) bool {
	_, exists := m.participants[username]
	return exists
}

func (m *MemoryBackend) doesChannelExist(channelName string) bool {
	_, exists := m.channels[channelName]
	return exists
}

func (b *MemoryBackend) HasParticipant(username string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.doesParticipantExist(username)
}

func (m *MemoryBackend) RegisterParticipant(participant *types.Participant) {
	m.mu.Lock()
	defer m.mu.Unlock() // deferred in case of panic

	if m.doesParticipantExist(participant.Username) {
		log.Logger.Panic("Participant %s already exists", participant.Username)
	}

	passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
	if !validation.ValidatePasswordSha256(passwordHash) {
		log.Logger.Panic("Password hash validation failed")
	}

	m.participants[participant.Username] = &types.Participant{
		Username: participant.Username,
		Password: passwordHash,
		Email:    participant.Email,
		JoinTime: participant.JoinTime,
	}

	log.Logger.Info("Registered %s participant", participant.Username)
}

func (m *MemoryBackend) AuthParticipant(participant *types.Participant) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	participant, exists := m.participants[participant.Username]
	if exists {
		passwordHash := utilities.Sha256Checksum([]byte(participant.Password))
		return strings.EqualFold(participant.Password, passwordHash)
	}

	return false
}

func (m *MemoryBackend) StoreMessage(message *types.ChatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := &types.ChatMessage{
		Contents: bytes.NewBuffer(bytes.Clone(message.Contents.Bytes())),
		Sender:   message.Sender,
		Channel:  message.Channel,
		Time:     message.Time,
	}

	if message.Channel != "" {
		channel, exists := m.channels[message.Channel]
		if !exists {
			log.Logger.Panic("Failed to store a message, channel %s doesn't exist", message.Channel)
		}

		channel.ChatHistory = append(channel.ChatHistory, msg)
		log.Logger.Info("Added messages to %s channel", channel.Name)
	} else {
		m.chatHistory = append(m.chatHistory, msg)
		log.Logger.Info("Added message to general channel")
	}
}

func (m *MemoryBackend) HasChannel(channelname string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doesChannelExist(channelname)
}

func (m *MemoryBackend) RegisterChannel(channel *types.Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.doesChannelExist(channel.Name) {
		log.Logger.Panic("Channel %s already exists", channel.Name)
	}

	m.channels[channel.Name] = &types.Channel{
		Name:         channel.Name,
		Desc:         channel.Desc,
		Creator:      channel.Creator,
		CreationDate: channel.CreationDate,
		ChatHistory:  make([]*types.ChatMessage, 0, 1024),
		Members:      make([]string, 0, 1024),
	}

	log.Logger.Info("Registered %s channel", channel.Name)
}

func (m *MemoryBackend) DeleteChannel(channelname string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.doesChannelExist(channelname) {
		log.Logger.Panic("Deletion failed, channel %s doesn't exist", channelname)
	}
	delete(m.channels, channelname)

	log.Logger.Info("Deleted %s channel", channelname)

	return true
}

func (m *MemoryBackend) GetChatHistory() []*types.ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.chatHistory
}

func (m *MemoryBackend) GetChannelHistory(channelname string) []*types.ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.doesChannelExist(channelname) {
		log.Logger.Panic("Failed to list chat history, channel %s doesn't exist", channelname)
	}

	channel := m.channels[channelname]
	return channel.ChatHistory
}

func (m *MemoryBackend) GetChannels() []*types.Channel {
	m.mu.Lock()
	defer m.mu.Unlock()

	var channels []*types.Channel
	var chanCount = len(m.channels)

	if chanCount != 0 {
		channels = make([]*types.Channel, 0, chanCount)
		for _, ch := range m.channels {
			channels = append(channels, ch)
		}
	}
	return channels
}

func (m *MemoryBackend) GetParticipantList() []*types.Participant {
	m.mu.Lock()
	defer m.mu.Unlock()

	var partList []*types.Participant
	var partCount = len(m.participants)

	if partCount != 0 {
		partList = make([]*types.Participant, 0, partCount)
		for _, participant := range m.participants {
			partList = append(partList, participant)
		}
	}
	return partList
}
