package session

import (
	"fmt"
	"strings"
)

func buildMenu(session *Session) string {
	builder := strings.Builder{}
	builder.Grow((len(menuOptionsTable) * 64) + 32)

	builder.WriteString("Options:\r\n")

	for idx, op := range menuOptionsTable {
		builder.WriteString(fmt.Sprintf("\t[%d] %s\r\n", idx+1, op))
	}

	return builder.String()
}

func buildChannelList(session *Session) (bool, string) {
	channels := session.storage.GetChannels()
	channelsCount := len(channels)

	if channelsCount == 0 {
		return false, ""
	}

	builder := strings.Builder{}
	builder.Grow((channelsCount * 64) + 64)
	builder.WriteString("Channels:\r\n")

	for index, ch := range channels {
		builder.WriteString(fmt.Sprintf("\t[%d] %s\r\n", index, ch.Name))
	}

	return true, builder.String()
}

func buildChatHistory(session *Session, channel ...string) (bool, string) {
	chatHistory := session.storage.GetChatHistory(channel...)
	chatLen := len(chatHistory)

	if chatLen == 0 {
		return false, ""
	}

	totalSize := 0
	for _, msg := range chatHistory {
		totalSize += len(msg.Contents)
		totalSize += len(msg.Sender)
		totalSize += len(msg.Time)
		totalSize += 4 // for '[:] ' characters
	}

	builder := strings.Builder{}
	builder.Grow(totalSize)

	for _, msg := range chatHistory {
		fullMsg := fmt.Sprintf("[%s:%s] %s", msg.Sender, msg.Time, msg.Contents)
		builder.WriteString(fullMsg)
	}

	return true, builder.String()
}

func buildParticipantList(session *Session) (bool, string) {
	participants := session.storage.GetParticipantList()
	participantsCount := len(participants)

	if participantsCount == 0 {
		return false, ""
	}

	builder := strings.Builder{}
	builder.Grow((participantsCount * 128) + 64)

	builder.WriteString("Participants:\r\n")

	// Iterate over all the participants in a storage,
	// check whether they are in a connections map to verify which status to display
	// `online` or `offline`. If a paticipant is present in a connections map
	// and its status is not Pending, that it is online, offline otherwise.
	// If the state is Pending, a participant is considered to still be offline.
	for _, p := range participants {
		if session.connections.isParticipantConnected(p.Name) {
			builder.WriteString(fmt.Sprintf("\t|%-64s| *%s", p.Name, connectionStateTable[1]))
			continue
		}

		builder.WriteString(fmt.Sprintf("\t|%-64s| *%s", p.Name, connectionStateTable[0]))
	}

	return true, builder.String()
}
