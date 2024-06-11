package commands

import (
	"bytes"
	"strings"

	"github.com/isnastish/chat/pkg/utilities"
)

type CommandType int8

const (
	CommandDisplayMenu    CommandType = 0
	CommandDisplayHistory CommandType = 0x1
	CommandListMembers    CommandType = 0x2
	CommandListChannels   CommandType = 0x3
	CommandListCommands   CommandType = 0x4

	// TODO: Document
	_CommandTerminal CommandType = 0x5
)

type ParseResult struct {
	CommandType
	Channel      string
	TimeDuration string
}

type command struct {
	name string
	desc string
	args []string
}

var commandTable []*command

var CommandsBuilder strings.Builder

func newCommand(name, desc string, args ...string) *command {
	return &command{
		name: name,
		desc: desc,
		args: args,
	}
}

func init() {
	commandTable = make([]*command, 0, _CommandTerminal-CommandDisplayMenu+1)

	commandTable[CommandDisplayMenu] = newCommand(":menu", "Display menu")
	commandTable[CommandDisplayHistory] = newCommand(":history", "Display chat history", "<channel>", "<period>")
	commandTable[CommandListMembers] = newCommand(":members", "Display chat members", "<channel>")
	commandTable[CommandListChannels] = newCommand(":channels", "Display all channels")
	commandTable[CommandListCommands] = newCommand(":commands", "Display commands")

	CommandsBuilder = strings.Builder{}
	CommandsBuilder.WriteString("commands:\n")

	for _, cmd := range commandTable {
		cmdstr := util.Fmtln("\t%s %s\t%s", cmd.name, strings.Join(cmd.args, " "), cmd.desc)
		CommandsBuilder.WriteString(cmdstr)
	}
}

func ParseCommand(buf *bytes.Buffer) (*ParseResult, bool) {
	return &ParseResult{}, false
}
