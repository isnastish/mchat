// TODO: Better error messages
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
	CommandNull CommandType = iota
	CommandDisplayMenu
	CommandDisplayHistory
	CommandListMembers
	CommandListChannels
	CommandListCommands

	// This type should always be the last
	commandSentinel
)

type errorType int8

const (
	errorSuccess errorType = iota
	errorUnexpectedArgument
	errorArgumentNotSpecified
	errorInvalidValue
	errorUnrecognizedArgument

	// This type should always be the last
	errorSentinel
)

type parseError struct {
	t   errorType
	msg string
}

type ParseResult struct {
	CommandType
	Channel string
	Period  uint
	Error   *parseError
	Matched bool
}

type option struct {
	name string
	arg  string
	hint string
}

type command struct {
	_type   CommandType
	name    string
	desc    string
	options []*option
}

var errorsTable []string
var commandTable []*command
var CommandsBuilder strings.Builder

func newCommand(cmdtype CommandType, name, desc string, options ...*option) *command {
	return &command{
		_type:   cmdtype,
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
	if cmd <= CommandNull || cmd >= commandSentinel {
		log.Logger.Panic("Index out of range")
	}
	return int(cmd - CommandDisplayMenu)
}

func (e *parseError) Error() string {
	if e.t <= errorSuccess || e.t >= errorSentinel {
		log.Logger.Panic("Index out of range")
	}
	return util.Fmtln("error: %s", errorsTable[e.t]+" "+e.msg)
}

func init() {
	commandTable = make([]*command, commandSentinel-CommandNull-1)

	commandTable[index(CommandDisplayMenu)] = newCommand(CommandDisplayMenu, ":menu", "Display menu")
	commandTable[index(CommandDisplayHistory)] =
		newCommand(CommandDisplayHistory, ":history", "Display chat history").
			addOption("-channel", "<name>", "Channel's name").
			addOption("-period", "<n>", "Time period")
	commandTable[index(CommandListMembers)] =
		newCommand(CommandListMembers, ":members", "Display chat members").
			addOption("-channel", "<name>", "Channel's name")
	commandTable[index(CommandListChannels)] = newCommand(CommandListChannels, ":channels", "Display all channels")
	commandTable[index(CommandListCommands)] = newCommand(CommandListCommands, ":commands", "Display commands")

	errorsTable = make([]string, errorSentinel-errorSuccess)
	errorsTable[errorSuccess] = "Success"
	errorsTable[errorUnexpectedArgument] = "Unexpected argument"
	errorsTable[errorArgumentNotSpecified] = "Argument not specified"
	errorsTable[errorInvalidValue] = "Invalid value"
	errorsTable[errorUnrecognizedArgument] = "Unregognized argument"

	CommandsBuilder = strings.Builder{}
	CommandsBuilder.WriteString("commands:\n")

	for _, cmd := range commandTable {
		CommandsBuilder.WriteString(util.Fmtln("\t%-20s\t%s", cmd.name, cmd.desc))
		for _, opt := range cmd.options {
			CommandsBuilder.WriteString(util.Fmtln("\t%-20s\t%s %s %s", "", opt.name, opt.arg, opt.hint))
		}
		CommandsBuilder.WriteString("\r\n")
	}
}

func ParseCommand(buffer *bytes.Buffer) *ParseResult {
	result := &ParseResult{CommandType: commandSentinel}
	if strings.HasPrefix(buffer.String(), ":") {
		arguments := strings.Split(buffer.String(), " ")
		for _, cmd := range commandTable {
			if strings.ToLower(arguments[0]) == cmd.name {
				result.CommandType = cmd._type
				optionIndex := 0
				for i := 1; i < len(arguments); i += 2 {
					if optionIndex >= len(cmd.options) {
						result.Error = &parseError{
							t:   errorUnexpectedArgument,
							msg: util.Fmt("%s", arguments[i])}
						return result
					}

					if i == len(arguments)-1 {
						result.Error = &parseError{t: errorArgumentNotSpecified}
						return result
					}

					for _, opt := range cmd.options {
						if arguments[i] == opt.name {
							if opt.name == "-channel" {
								result.Channel = arguments[i+1]
								result.Matched = true
								break

							} else if opt.name == "-period" {
								count, err := strconv.Atoi(arguments[i+1])
								if err != nil || count < 0 {
									result.Error = &parseError{
										t:   errorInvalidValue,
										msg: util.Fmt("period %s", arguments[i+1])}
									return result
								}

								result.Period = uint(count)
								result.Matched = true
								break
							}
						}
					}

					// The error is set when a command expects some arguments/options,
					// but none matched.
					if !result.Matched {
						result.Error = &parseError{
							t:   errorUnrecognizedArgument,
							msg: arguments[i],
						}
						return result
					}

					optionIndex++
				}
				result.Matched = true
				return result
			}
		}
	}

	return result
}
