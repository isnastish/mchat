package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func str2bytes(str string) *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, len(str)))
	buf.WriteString(str)
	return buf
}

func notACommand(t *testing.T, command string) {
	result := ParseCommand(str2bytes(command))
	assert.False(t, result.Matched)
}

func commandParseError(t *testing.T, command string, err errorType) {
	result := ParseCommand(str2bytes(command))
	assert.Equal(t, result.Error.t, err)
}

func validCommand(t *testing.T, command string, expectedtype CommandType) {
	result := ParseCommand(str2bytes(command))
	assert.True(t, result.Matched)
	assert.Equal(t, result.CommandType, expectedtype)
}

func TestNotCommands(t *testing.T) {
	notACommand(t, "not a command")
	notACommand(t, ":")
	notACommand(t, ":men")
	notACommand(t, ":menu___")
	notACommand(t, ":_commands")
	notACommand(t, "::")
	notACommand(t, ":members_still_no_a_command")
}

func TestErrorParsingACommand(t *testing.T) {
	commandParseError(t, ":commands unexpected_argument", errorUnexpectedArgument)
	commandParseError(t, ":history --unrecognized argument", errorUnrecognizedArgument)
	commandParseError(t, ":history -chan (still unrecognized argument)", errorUnrecognizedArgument)
	commandParseError(t, ":history -channel", errorArgumentNotSpecified)
	commandParseError(t, ":history -channel BooksChannel -period", errorArgumentNotSpecified)
	commandParseError(t, ":history -period -234", errorInvalidValue) // debug
	commandParseError(t, ":history -period string", errorInvalidValue)
	commandParseError(t, ":members -channel", errorArgumentNotSpecified)
	// nil on success
	// commandParseError(t, ":members -channel Books", errorSuccess)
}

func TestValidCommands(t *testing.T) {
	validCommand(t, ":commands", CommandListCommands)
	validCommand(t, ":menu", CommandDisplayMenu)
	validCommand(t, ":channels", CommandListChannels)
	validCommand(t, ":members", CommandListMembers)
	validCommand(t, ":history", CommandDisplayHistory)

	cmd := ":history -channel Dragonflies -period 8"
	buf := bytes.NewBuffer(make([]byte, 0, len(cmd)))
	buf.WriteString(cmd)
	result := ParseCommand(buf)
	assert.True(t, result.Matched)
	assert.Equal(t, CommandDisplayHistory, result.CommandType)
	assert.Equal(t, "Dragonflies", result.Channel)
	assert.Equal(t, uint(8), result.Period)

	cmd = ":history -period 12"
	buf.Truncate(0)
	buf.WriteString(cmd)
	result = ParseCommand(buf)
	assert.True(t, result.Matched)
	assert.Equal(t, CommandDisplayHistory, result.CommandType)
	assert.Equal(t, uint(12), result.Period)

	cmd = ":members -channel Books"
	buf.Truncate(0)
	buf.WriteString(cmd)
	result = ParseCommand(buf)
	assert.True(t, result.Matched)
	assert.Equal(t, CommandListMembers, result.CommandType)
	assert.Equal(t, "Books", result.Channel)
}
