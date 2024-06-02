// NOTE: Participant, Channle, Participant/SystemMessage structs could be moved to a separate package.
// That way we could reuse them easily, without pulling out the whole backend package.
// Or it might be a sub-package of a backend. Since now the client has to duplicate the messages,
// which could be avoided if we pull everything out into a seprate package.
// Backend should provide an api to store the data and all the types should be in a seprate package.
package backend

import (
	"github.com/isnastish/chat/pkg/types"
)

type BackendType int8

const (
	BackendTypeDynamodb BackendType = 0x01
	BackendTypeRedis    BackendType = 0x02
	BackendTypeMemory   BackendType = 0x03
)

type Backend interface {
	HasParticipant(username string) bool
	RegisterParticipant(participant *types.Participant)
	AuthParticipant(participant *types.Participant) bool
	StoreMessage(message *types.ChatMessage)
	HasChannel(channelname string) bool
	RegisterChannel(channel *types.Channel)
	DeleteChannel(channelname string) bool
	GetChatHistory() []*types.ChatMessage
	GetChannelHistory(channelname string) []*types.ChatMessage
	GetChannels() []*types.Channel
	GetParticipantList() []*types.Participant

	// ChanelAddMember(channelname string, member string)
}
