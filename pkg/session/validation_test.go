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
	assert.False(t, validatePassword("****-adff==#sdf989778A"))
	assert.False(t, validatePassword("/.well-known/acme-challenge"))
	assert.False(t, validatePassword("password2348"))
	assert.False(t, validatePassword("ThisPa2swordExceeeeeedsTheAllowedAmountOfCharacters"))
	assert.False(t, validatePassword(".a"))

	// valid passwords
	assert.True(t, validatePassword("Afdsf988#@Nasayer"))
	assert.True(t, validatePassword("2344NewYear@lone"))
	assert.True(t, validatePassword("NeverAgain1999#"))
}

func TestValidateName(t *testing.T) {
	// invalid names
	assert.False(t, validateName("Short"))
	assert.False(t, validateName("234StartsWithDigits"))
	assert.False(t, validateName("_StartWithUnderscore"))
	assert.False(t, validateName("Contains-***InvalidSymbols@"))
	assert.False(t, validateName("NameIsTooLong23449988AndExceeeedsTheDesiredSizeOf32Symbols"))
	assert.False(t, validateName("A.1"))

	// valid names
	assert.True(t, validateName("Hadson24499"))
	assert.True(t, validateName("nasayer_777"))
	assert.True(t, validateName("Humanoid4You_"))
}

func TestValidateEmailAddress(t *testing.T) {

}
