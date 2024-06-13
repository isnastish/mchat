package commands

import (
	"bytes"
	_ "strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func invalidCommand(command string, t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString(command)
	_, matched := ParseCommand(buf)
	assert.False(t, matched)
}

func validCommand(command string, commandType CommandType, t *testing.T) {
	// buf := bytes.NewBuffer(make([]byte, 0, 256))
	// buf.WriteString(command)
	// builder := strings.Builder{}
	// result, matched := ParseCommand()
	// assert.True(t, matched)
	// assert.Equal(t, result.CommandType, commandType)
}

func TestInvalidCommands(t *testing.T) {
	invalidCommand("not a command", t)
	invalidCommand(":", t)
	invalidCommand(":men", t)
	invalidCommand(":menu___", t)

	validCommand(":menu", CommandDisplayMenu, t)
	validCommand(":channels", CommandListChannels, t)
	validCommand(":commands", CommandListCommands, t)
}
