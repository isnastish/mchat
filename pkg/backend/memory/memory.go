package memory

import (
	"bytes"
	"sync"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/types"
	"github.com/isnastish/chat/pkg/utilities"
	"github.com/isnastish/chat/pkg/validation"
)

type memoryBackend struct {
	participants map[string]*types.Participant
	chatHistory  []*types.ChatMessage
	channels     map[string]*types.Channel
	sync.RWMutex
}

func NewMemoryBackend() *memoryBackend {
	return &memoryBackend{
		participants: make(map[string]*types.Participant),
		chatHistory:  make([]*types.ChatMessage, 0, 1024),
		channels:     make(map[string]*types.Channel),
	}
}

func (m *memoryBackend) doesParticipantExist(username string) bool {
	_, exists := m.participants[username]
	return exists
}

func (m *memoryBackend) doesChannelExist(channelName string) bool {
	_, exists := m.channels[channelName]
	return exists
}

func (m *memoryBackend) HasParticipant(username string) bool {
	m.RLock()
	defer m.RUnlock()
	return m.doesParticipantExist(username)
}

func (m *memoryBackend) RegisterParticipant(participant *types.Participant) {
	m.Lock()
	defer m.Unlock() // deferred in case of panic

	if m.doesParticipantExist(participant.Username) {
		log.Logger.Panic("Participant %s already exists", participant.Username)
	}

	passwordHash := util.Sha256Checksum([]byte(participant.Password))
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

func (m *memoryBackend) AuthParticipant(participant *types.Participant) bool {
	m.RLock()
	defer m.RUnlock()

	foundParticipant, exists := m.participants[participant.Username]
	if exists {
		passwordHash := util.Sha256Checksum([]byte(participant.Password))
		return passwordHash == foundParticipant.Password
	}

	return false
}

func (m *memoryBackend) StoreMessage(message *types.ChatMessage) {
	m.Lock()
	defer m.Unlock()

	msg := &types.ChatMessage{
		Contents: bytes.NewBuffer(bytes.Clone(message.Contents.Bytes())),
		Sender:   message.Sender,
		Channel:  message.Channel,
		SentTime: message.SentTime,
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

func (m *memoryBackend) HasChannel(channelname string) bool {
	m.RLock()
	defer m.RUnlock()
	return m.doesChannelExist(channelname)
}

func (m *memoryBackend) RegisterChannel(channel *types.Channel) {
	m.Lock()
	defer m.Unlock()

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

func (m *memoryBackend) DeleteChannel(channelname string) bool {
	m.Lock()
	defer m.Unlock()
	if !m.doesChannelExist(channelname) {
		log.Logger.Panic("Deletion failed, channel %s doesn't exist", channelname)
	}
	delete(m.channels, channelname)

	log.Logger.Info("Deleted %s channel", channelname)

	return true
}

func (m *memoryBackend) GetChatHistory(channelname ...string) []*types.ChatMessage {
	m.RLock()
	defer m.RUnlock()

	// Empty ("") channels name is treated the same as the channel not being specified,
	// thus we have to return general chat's history
	if len(channelname) > 0 && channelname[0] != "" {
		if !m.doesChannelExist(channelname[0]) {
			log.Logger.Panic("Failed to list chat history, channel %s doesn't exist", channelname)
		}
		channel := m.channels[channelname[0]]
		return channel.ChatHistory
	}
	return m.chatHistory
}

func (m *memoryBackend) GetChannels() []*types.Channel {
	m.RLock()
	defer m.RUnlock()

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

func (m *memoryBackend) GetParticipants() []*types.Participant {
	m.RLock()
	defer m.RUnlock()

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
