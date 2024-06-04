package testsetup

import (
	"bytes"

	"github.com/isnastish/chat/pkg/types"
)

var Participants = []types.Participant{
	{
		Username: "IvanIvanov",
		Password: "ThisIsI1sPassw0rd@",
		Email:    "example@gmail.com",
	},
	{
		Username: "MarkLutz",
		Password: "Some_other_password@234",
		Email:    "mark@mail.ru",
	},

	{
		Username: "nasayer1234",
		Password: "nasayerLive234@Outline",
		Email:    "nasayer@gmail.com",
	},
	{
		Username: "anonymus__",
		Password: "SomeRandomPass8ordHere",
		Email:    "anonymus@mail.ru",
	},
}

var Channels = []types.Channel{
	{
		Name:    "BooksChannel",
		Desc:    "Channel to shared sci-fy books",
		Creator: "Sarah Obrian",
	},
	{
		Name:    "ProgrammingChannel",
		Desc:    "Channel for sharing programming related things",
		Creator: "Anna Herman",
	},
}

// Messages sent in the BooksChannel.
var BooksChannelMessages = []types.ChatMessage{
	{
		Contents: message("Hi everyone, this is my first message in this channel, " +
			"I'm so exited about any kinds of books."),
		Sender:  "Mark Obrian",
		Channel: Channels[0].Name,
	},
	{
		Contents: message("Hi Mark! What kinds of books have you read so far? We all cannot wait to hear." +
			"Fill free to post it in the channel."),
		Sender:  "Olivia",
		Channel: Channels[0].Name,
	},
	{
		Contents: message("Nice to meet you Mark!!!"),
		Sender:   "John Nilson",
		Channel:  Channels[0].Name,
	},
}

// Messages sent in the ProgrammingChannel.
var ProgrammingChannelMessages = []types.ChatMessage{
	{
		Contents: message("H! I'm a new joiner. I'm passionate about Go programming language"),
		Sender:   "Myself",
		Channel:  Channels[1].Name,
	},
	{
		Contents: message("Hello. That's great to hear, what other languages you're passionate about?" +
			"I've heard that you're pretty good at C++ and Python as well."),
		Sender:  "Niklas",
		Channel: Channels[1].Name,
	},
	{
		Contents: message("Well yes, you're absolutely right, I'm fluent in C++ and Python as well, " +
			"but currently I write all my hobby projects in Golang. Since I want to focus on cloud native development."),
		Sender:  "Myself",
		Channel: Channels[1].Name,
	},
}

// Messages sent in a general chat.
var GeneralMessages = []types.ChatMessage{
	{
		Contents: message("Hello! It has been a while since we met for the last time." +
			"I was missing you a lot and cannot wait to spend some time together."),
		Sender: "Elizabeth Swann",
	},
}

// Helper function for creating message contents.
// The size of the buffer should be increased to allow testing of enormously large messages.
func message(msg string) *bytes.Buffer {
	nBytes := len(msg)
	buf := bytes.NewBuffer(make([]byte, 0, nBytes))
	buf.WriteString(msg)
	return buf
}

func ContainsMessage(src []types.ChatMessage, message *types.ChatMessage) bool {
	for _, srcMsg := range src {
		if srcMsg.Channel == message.Channel &&
			srcMsg.Sender == message.Sender &&
			srcMsg.Time == message.Time &&
			bytes.Equal(srcMsg.Contents.Bytes(), message.Contents.Bytes()) {
			return true
		}
	}
	return false
}

func ContainsParticipant(src []types.Participant, p *types.Participant) bool {
	for _, srcP := range src {
		if srcP.Username == p.Username &&
			srcP.Email == p.Email &&
			srcP.JoinTime == p.JoinTime { // Match the password as well?
			return true
		}
	}
	return false
}

func ContainsChannel(src []types.Channel, ch *types.Channel) bool {
	for _, srcCh := range src {
		// TODO: Match chat messages and members as well.
		// Maybe a channel should hold chat messages.
		// I would have to think about it more carefully
		if srcCh.Name == ch.Name &&
			srcCh.Desc == ch.Desc &&
			srcCh.Creator == ch.Creator &&
			srcCh.CreationDate == ch.CreationDate {
			return true
		}
	}
	return false
}

type dataType interface {
	types.ChatMessage | types.Participant | types.Channel
}

// TODO: Test modifying []*types.ChatMessage vs []types.ChatMessage,
// if any modications caused to []types.ChatMessage modify the origin slice,
// we shouldn't pass pointer everywhere.
// Unify Match function to match Participants, Channels and Messages.
// And move contanins function outside.
func Match[T dataType](a []*T, b []T, callback func([]T, *T) bool) bool {
	if len(a) != len(b) {
		return false
	}

	for _, item := range a {
		if !callback(b, item) {
			return false
		}
	}

	return true
}
