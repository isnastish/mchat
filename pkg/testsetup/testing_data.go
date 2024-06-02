package testsetup

import (
	"bytes"

	"github.com/isnastish/chat/pkg/types"
)

// Helper function for creating message contents.
// The size of the buffer should be increased to allow testing of enormously large messages.
func message(msg string) *bytes.Buffer {
	nBytes := len(msg)
	buf := bytes.NewBuffer(make([]byte, 0, nBytes))
	buf.WriteString(msg)
	return buf
}

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
