package commands

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/isnastish/chat/pkg/logger"
	"github.com/isnastish/chat/pkg/utilities"
)

type CommandType int8

const (
	CommandNull                       = 0
	CommandDisplayMenu    CommandType = 0x1
	CommandDisplayHistory CommandType = 0x2
	CommandListMembers    CommandType = 0x3
	CommandListChannels   CommandType = 0x4
	CommandListCommands   CommandType = 0x5

	_Sentinel CommandType = 0x6
)

type ParseResult struct {
	CommandType
	Channel string
	Period  uint
	Error   error
}

type option struct {
	name string
	arg  string
	hint string
}

type command struct {
	name    string
	desc    string
	options []*option
	_type   CommandType
}

var commandTable []*command
var CommandsBuilder strings.Builder

func newCommand(name, desc string, options ...*option) *command {
	return &command{
		name:    name,
		desc:    desc,
		options: options,
	}
}

func (c *command) addOption(name, arg, hint string) *command {
	c.options = append(c.options, &option{name: name, arg: arg, hint: hint})
	return c
}

func index(cmd CommandType) int {
	if cmd <= CommandNull || cmd >= _Sentinel {
		log.Logger.Panic("Index out of range")
	}
	return int(cmd - CommandDisplayMenu)
}

func init() {
	commandTable = make([]*command, _Sentinel-CommandNull-1)

	commandTable[index(CommandDisplayMenu)] = newCommand(":menu", "Display menu")
	commandTable[index(CommandDisplayHistory)] =
		newCommand(":history", "Display chat history").
			addOption("-channel", "<name>", "Channel's name").
			addOption("-period", "<n>", "Time period")
	commandTable[index(CommandListMembers)] =
		newCommand(":members", "Display chat members").
			addOption("-channel", "<name>", "Channel's name")
	commandTable[index(CommandListChannels)] = newCommand(":channels", "Display all channels")
	commandTable[index(CommandListCommands)] = newCommand(":commands", "Display commands")

	CommandsBuilder = strings.Builder{}
	CommandsBuilder.WriteString("commands:\n")

	for _, cmd := range commandTable {
		CommandsBuilder.WriteString(util.Fmtln("%-20s\t%s", cmd.name, cmd.desc))
		for _, opt := range cmd.options {
			CommandsBuilder.WriteString(util.Fmtln("%-20s\t%s %s %s", "", opt.name, opt.arg, opt.hint))
		}
		CommandsBuilder.WriteString("\r\n")
	}
}

func ParseCommand(buffer *bytes.Buffer) (*ParseResult, bool) {
	builder := strings.Builder{}
	builder.WriteString(buffer.String())

	if strings.HasPrefix(builder.String(), ":") {
		arguments := strings.Split(builder.String(), " ")
		result := &ParseResult{CommandType: _Sentinel}
		for _, cmd := range commandTable {
			if strings.ToLower(arguments[0]) == cmd.name {
				result.CommandType = cmd._type
				for i := 1; i < len(arguments); i += 2 { // skip the first argument
					if i > len(cmd.options) {
						// result.Error = util.ErrorF("Unrecognized option %s", arguments[i])
						result.Error = util.ErrorF("Unexpected option")
						return result, true
					}

					// was the last argument
					if i == len(arguments)-1 {
						result.Error = util.ErrorF("Argument not specified")
						return result, true
					}

					// iterate over all the options since they might be in a different order
					for _, opt := range cmd.options {
						if arguments[i] == opt.name {
							if opt.name == "-channel" {
								result.Channel = arguments[i+1]
								break

							} else if opt.name == "-period" {
								count, err := strconv.Atoi(arguments[i+1])
								if err != nil {
									result.Error = util.ErrorF("Invalid period %s", arguments[i+1])

								} else if count < 0 {
									result.Error = util.ErrorF("")
								}

								result.Period = uint(count)
							}
						}
					}
				}
				return result, true
			}
		}
	}
	return nil, false
}
