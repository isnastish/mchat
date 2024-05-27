package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.False(t, validateName("")) // too short

	// valid passwords
	assert.True(t, validatePassword("Afdsf988#@Nasayer"))
	assert.True(t, validatePassword("2344NewYear@lone"))
	assert.True(t, validatePassword("NeverAgain1999#"))
	assert.True(t, validateName("a")) // valid username, a single character
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
	assert.False(t, validateEmailAddress("John..Doe@example.com"))
	assert.False(t, validateEmailAddress("abc.example.com"))
	assert.False(t, validateEmailAddress("i.like.underscores@but_they_are_not_allowed_in_this_part"))                       // underscore is not allowed in domain part
	assert.False(t, validateEmailAddress("a@b@c@example.com"))                                                              // only one @ is allowed outside quotation marks
	assert.False(t, validateEmailAddress(`a"b(c)d,e:f;g<h>i[j\k]l@example.com`))                                            // none of the special characters in this local-part are allowed outside quotation marks
	assert.False(t, validateEmailAddress(`just"not"right@example.com`))                                                     // quoted strings must be dot separated or be the only element making up the local-part
	assert.False(t, validateEmailAddress(`this is"not\allowed@example.com`))                                                // spaces, quotes, and backslashes may only exist when within quoted strings and preceded by a backslash
	assert.False(t, validateEmailAddress(`this\ still\"not\\allowed@example.com`))                                          // even if escaped (preceded by a backslash), spaces, quotes, and backslashes must still be contained by quotes
	assert.False(t, validateEmailAddress(`1234567890123456789012345678901234567890123456789012345678901234+x@example.com`)) // local-part is longer than 64 characters

	// valid names
	assert.True(t, validateEmailAddress("John.Doe@example.com"))
	assert.True(t, validateEmailAddress("simple@example.com"))
	assert.True(t, validateEmailAddress("very.common@example.com"))
	assert.True(t, validateEmailAddress("FirstName.LastName@EasierReading.org")) // case is always ignored after the @ and usually before
	assert.True(t, validateEmailAddress("x@example.com"))                        // one-letter local-part
	assert.True(t, validateEmailAddress("long.email-address-with-hyphens@and.subdomains.example.com"))
	assert.True(t, validateEmailAddress("user.name+tag+sorting@example.com"))                         // may be routed to user.name@example.com inbox depending on mail server
	assert.True(t, validateEmailAddress("name/surname@example.com"))                                  // slashes are a printable character, and allowed
	assert.True(t, validateEmailAddress("admin@example"))                                             // local domain name with no TLD, although ICANN highly discourages dotless email addresses[29]
	assert.True(t, validateEmailAddress("example@s.example"))                                         // see the List of Internet top-level domains
	assert.True(t, validateEmailAddress("mailhost!username@example.org"))                             // bangified host route used for uucp mailers
	assert.True(t, validateEmailAddress("user%example.com@example.org"))                              // % escaped mail r"oute to user@example.com via example.org)
	assert.True(t, validateEmailAddress("user-@example.org"))                                         // local-part ending with non-alphanumeric character from the list of allowed printable characters)
	assert.True(t, validateEmailAddress("postmaster@[123.123.123.123]"))                              // IP addresses are allowed instead of domains when in square brackets, but strongly discouraged)
	assert.True(t, validateEmailAddress("postmaster@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334]")) // IPv6 uses a different syntax
	assert.True(t, validateEmailAddress("_test@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334]"))      // begin with underscore different syntax

	// TODO(alx): Add tests for quoted email addresses once the parsing is supported.
	// "@example.org (space between the quotes)
	// "john..doe"@example.org (quoted double dot)
	// "very.(),:;<>[]\".VERY.\"very@\\ \"very\".unusual"@strange.example.com (include non-letters character AND multiple at sign, the first one being double quoted)
}
