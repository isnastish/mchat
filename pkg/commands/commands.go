// TODO: Register command arguments and then match them rather than doing manual parsing
// :history --channel BooksChannel --period 8 (days) (maybe this will be more intuitive and simplify parsing)
// :members --channel ProgrammingChannel
package commands

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/isnastish/chat/pkg/utilities"
)

type CommandType int8

const (
	// CommandNull = 0
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
	Channel string
	Period  uint
	Error   error
}

type command struct {
	name  string
	desc  string
	args  []string
	_type CommandType
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
	commandTable = make([]*command, _CommandTerminal-CommandDisplayMenu)

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

// TODO: Pass a string builder?
func ParseCommand(buf *bytes.Buffer) (*ParseResult, bool) {
	if strings.HasPrefix(buf.String(), ":") {
		arguments := strings.Split(buf.String(), " ")
		result := &ParseResult{CommandType: _CommandTerminal}
		for _, cmd := range commandTable {
			if arguments[0] == cmd.name {
				result.CommandType = cmd._type
				if len(arguments) > 1 {

					switch cmd._type {
					case CommandDisplayHistory:
						result.Channel = arguments[1]
						if len(arguments) > 2 {
							// NOTE: This functionality is not supported by the backend itself yet.
							// Because the backend cannot filter messages based on the time period.
							period, err := strconv.Atoi(arguments[2])
							if err != nil || period < 0 {
								result.Error = util.ErrorF("Argument %s doesn't match a time period", arguments[2])
							} else {
								result.Period = uint(period)
							}
						}
					case CommandListMembers:
						result.Channel = arguments[1]

					default:
						result.Error = util.ErrorF("Command %s doesn't accept arguments", cmd.name)
					}
				}
				return result, true
			}
		}
	}
	return nil, false
}
