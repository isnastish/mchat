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
	// invalid names
	// John..Doe@example.com

	// valid names
	// 	simple@example.com
	// very.common@example.com
	// FirstName.LastName@EasierReading.org (case is always ignored after the @ and usually before)
	// x@example.com (one-letter local-part)
	// long.email-address-with-hyphens@and.subdomains.example.com
	// user.name+tag+sorting@example.com (may be routed to user.name@example.com inbox depending on mail server)
	// name/surname@example.com (slashes are a printable character, and allowed)
	// admin@example (local domain name with no TLD, although ICANN highly discourages dotless email addresses[29])
	// example@s.example (see the List of Internet top-level domains)
	// " "@example.org (space between the quotes)
	// "john..doe"@example.org (quoted double dot)
	// mailhost!username@example.org (bangified host route used for uucp mailers)
	// "very.(),:;<>[]\".VERY.\"very@\\ \"very\".unusual"@strange.example.com (include non-letters character AND multiple at sign, the first one being double quoted)
	// user%example.com@example.org (% escaped mail route to user@example.com via example.org)
	// user-@example.org (local-part ending with non-alphanumeric character from the list of allowed printable characters)
	// postmaster@[123.123.123.123] (IP addresses are allowed instead of domains when in square brackets, but strongly discouraged)
	// postmaster@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334] (IPv6 uses a different syntax)
	// _test@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334] (begin with underscore different syntax)
}
