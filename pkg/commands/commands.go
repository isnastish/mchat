package commands

import (
	"bytes"
	_ "strings"
)

type Command int8

const (
	CommandDisplayMenu    Command = 0
	CommandDisplayHistory Command = 0x1
	CommandListMembers    Command = 0x2
	CommandListChannels   Command = 0x3
	CommandListCommands   Command = 0x4

	// TODO: Document
	_CommandTerminal Command = 0x5
)

type command struct {
	name string
	desc string
	args []string
}

func newCommand(name, desc string, args ...string) *command {
	return &command{
		name: name,
		desc: desc,
		args: args,
	}
}

var commandTable []*command

func init() {
	commandTable = make([]*command, 0, _CommandTerminal-CommandDisplayMenu+1)

	commandTable[CommandDisplayMenu] = newCommand(":menu", "Display menu")
	commandTable[CommandDisplayHistory] = newCommand(":history", "Display chat history", "<channel>", "<period>")
	commandTable[CommandListMembers] = newCommand(":members", "Display chat members", "<channel>")
	commandTable[CommandListChannels] = newCommand(":channels", "Display all channels")
	commandTable[CommandListCommands] = newCommand(":commands", "Display commands")
}

func ParseCommand(buf *bytes.Buffer) (Command, bool) {

	return _CommandTerminal, false
}
