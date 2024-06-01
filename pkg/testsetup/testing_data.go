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
		Creator: "Anna Hermann",
	},
}

func message(msg string) *bytes.Buffer {
	// TODO: Bump the buffer's size for testing enormously large messages.
	buf := bytes.NewBuffer(make([]byte, 256))
	buf.WriteString(msg)
	return buf
}

// Messages not belonging to any channel
var GeneralMessages = []types.ChatMessage{
	{
		Contents: message("Hello! It has been a while since we met for the last time." +
			"I was missing you a lot and cannot wait to spend some time together."),
		Sender: "Elizabeth Swann",
	},
	{},

	// {
	// 	Contents: []byte("Hello! My name is Anna."),
	// 	Sender:   "Anna",
	// },
	// {
	// 	Contents: []byte("Hi! How are you doing Anna?"),
	// 	Sender:   "Elizabeth",
	// },
	// {
	// 	Contents: []byte("Hi everyone, I was reading an interesting novel relatively recently and want to share some thought with you."),
	// 	Sender:   "Herman",
	// 	Channel:  "books_channel",
	// },
}

// Messages belonging to a concrete channel
var ChannelMessages = []types.ChatMessage{}
