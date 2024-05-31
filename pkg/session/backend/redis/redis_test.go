package redis

import (
	_ "github.com/stretchr/testify/assert"
	"testing"
)

func TestMain(m *testing.M) {
	// Run redis inside a docker container here.
}

func TestRegisterParticipant(t *testing.T) {
	storage := NewBackend()
	_ = storage
	// storage.RegisterParticipant()
}
