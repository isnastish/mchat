package session

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	// invalid passwords
	assert.False(t, validatePassword("onlylowercaseletters"))
	assert.False(t, validatePassword("ONLYUPPERCASELETTERS"))
	assert.False(t, validatePassword("tooShort"))
	assert.False(t, validatePassword("short3A@"))
	assert.False(t, validatePassword("244"))
	assert.False(t, validatePassword("23349999934443444"))
	assert.False(t, validatePassword("****-adff==#sdf989778"))
	assert.False(t, validatePassword("/.well-known/acme-challenge"))
	assert.False(t, validatePassword("password2348"))

	// valid passwords
	assert.True(t, validatePassword("Afdsf988#@Nasayer"))
	assert.True(t, validatePassword("2344NewYear@lone"))
	assert.True(t, validatePassword("NeverAgain1999#"))
}
