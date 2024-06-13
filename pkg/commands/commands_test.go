package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func str2bytes(str string) *bytes.Buffer {
	buf := bytes.NewBuffer(make([]byte, 0, len(str)))
	buf.WriteString(buf.String())
	return buf
}

func notACommand(command string, t *testing.T) {
	result := ParseCommand(str2bytes(command))
	assert.False(t, result.Matched)
}

func TestNotCommands(t *testing.T) {
	notACommand("not a command", t)
	notACommand(":", t)
	notACommand(":men", t)
	notACommand(":menu___", t)
	notACommand(":_commands", t)
	notACommand("::", t)
	notACommand(":members_still_no_a_command", t)
}

func TestErrorParsingACommand(t *testing.T) {
	result := ParseCommand(str2bytes(":commands unexpected_argument"))
	assert.Equal(t, result.Error.t, errorUnexpectedArgument)
}
