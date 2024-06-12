package commands

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func invalidCommand(command string, t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString(command)
	_, matched := ParseCommand(buf)
	assert.False(t, matched)
}

func validCommand(command string, commandType CommandType, t *testing.T) *ParseResult {
	buf := bytes.NewBuffer(make([]byte, 0, 256))
	buf.WriteString(command)
	result, matched := ParseCommand(buf)
	assert.True(t, matched)
	return result
}

func TestInvalidCommands(t *testing.T) {
	invalidCommand("not a command", t)
	invalidCommand(":", t)
	invalidCommand(":men", t)
	invalidCommand(":menu___", t)

	// cmd := validCommand(":menu", t)
}
