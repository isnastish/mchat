package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/isnastish/chat/pkg/utilities"
)

func TestValidatePassword(t *testing.T) {
	// invalid passwords
	assert.False(t, ValidatePassword("onlylowercaseletters"))
	assert.False(t, ValidatePassword("ONLYUPPERCASELETTERS"))
	assert.False(t, ValidatePassword("tooShort"))
	assert.False(t, ValidatePassword("short3A@"))
	assert.False(t, ValidatePassword("244"))
	assert.False(t, ValidatePassword("23349999934443444"))
	assert.False(t, ValidatePassword("****-adff==#sdf989778A"))
	assert.False(t, ValidatePassword("/.well-known/acme-challenge"))
	assert.False(t, ValidatePassword("password2348"))
	assert.False(t, ValidatePassword("ThisPa2swordExceeeeeedsTheAllowedAmountOfCharacters"))
	assert.False(t, ValidatePassword(".a"))

	// valid passwords
	assert.True(t, ValidatePassword("Afdsf988#@Nasayer"))
	assert.True(t, ValidatePassword("2344NewYear@lone"))
	assert.True(t, ValidatePassword("NeverAgain1999#"))
}

func TestValidateName(t *testing.T) {
	// invalid names
	assert.False(t, ValidateName("Short"))
	assert.False(t, ValidateName("234StartsWithDigits"))
	assert.False(t, ValidateName("_StartWithUnderscore"))
	assert.False(t, ValidateName("Contains-***InvalidSymbols@"))
	assert.False(t, ValidateName("NameIsTooLong23449988AndExceeeedsTheDesiredSizeOf32Symbols"))
	assert.False(t, ValidateName("A.1"))
	assert.False(t, ValidateName("invalid_username#"))

	// valid names
	assert.True(t, ValidateName("Hadson24499"))
	assert.True(t, ValidateName("nasayer_777"))
	assert.True(t, ValidateName("Humanoid4You_"))
}

func TestValidateEmailAddress(t *testing.T) {
	// invalid names
	assert.False(t, ValidateEmail("John..Doe@example.com"))
	assert.False(t, ValidateEmail("abc.example.com"))
	assert.False(t, ValidateEmail("i.like.underscores@but_they_are_not_allowed_in_this_part"))                       // underscore is not allowed in domain part
	assert.False(t, ValidateEmail("a@b@c@example.com"))                                                              // only one @ is allowed outside quotation marks
	assert.False(t, ValidateEmail(`a"b(c)d,e:f;g<h>i[j\k]l@example.com`))                                            // none of the special characters in this local-part are allowed outside quotation marks
	assert.False(t, ValidateEmail(`just"not"right@example.com`))                                                     // quoted strings must be dot separated or be the only element making up the local-part
	assert.False(t, ValidateEmail(`this is"not\allowed@example.com`))                                                // spaces, quotes, and backslashes may only exist when within quoted strings and preceded by a backslash
	assert.False(t, ValidateEmail(`this\ still\"not\\allowed@example.com`))                                          // even if escaped (preceded by a backslash), spaces, quotes, and backslashes must still be contained by quotes
	assert.False(t, ValidateEmail(`1234567890123456789012345678901234567890123456789012345678901234+x@example.com`)) // local-part is longer than 64 characters

	// valid names
	assert.True(t, ValidateEmail("John.Doe@example.com"))
	assert.True(t, ValidateEmail("simple@example.com"))
	assert.True(t, ValidateEmail("very.common@example.com"))
	assert.True(t, ValidateEmail("FirstName.LastName@EasierReading.org")) // case is always ignored after the @ and usually before
	assert.True(t, ValidateEmail("x@example.com"))                        // one-letter local-part
	assert.True(t, ValidateEmail("long.email-address-with-hyphens@and.subdomains.example.com"))
	assert.True(t, ValidateEmail("user.name+tag+sorting@example.com"))                         // may be routed to user.name@example.com inbox depending on mail server
	assert.True(t, ValidateEmail("name/surname@example.com"))                                  // slashes are a printable character, and allowed
	assert.True(t, ValidateEmail("admin@example"))                                             // local domain name with no TLD, although ICANN highly discourages dotless email addresses[29]
	assert.True(t, ValidateEmail("example@s.example"))                                         // see the List of Internet top-level domains
	assert.True(t, ValidateEmail("mailhost!username@example.org"))                             // bangified host route used for uucp mailers
	assert.True(t, ValidateEmail("user%example.com@example.org"))                              // % escaped mail r"oute to user@example.com via example.org)
	assert.True(t, ValidateEmail("user-@example.org"))                                         // local-part ending with non-alphanumeric character from the list of allowed printable characters)
	assert.True(t, ValidateEmail("postmaster@[123.123.123.123]"))                              // IP addresses are allowed instead of domains when in square brackets, but strongly discouraged)
	assert.True(t, ValidateEmail("postmaster@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334]")) // IPv6 uses a different syntax
	assert.True(t, ValidateEmail("_test@[IPv6:2001:0db8:85a3:0000:0000:8a2e:0370:7334]"))      // begin with underscore different syntax

	// TODO(alx): Add tests for quoted email addresses once the parsing is supported.
	// "@example.org (space between the quotes)
	// "john..doe"@example.org (quoted double dot)
	// "very.(),:;<>[]\".VERY.\"very@\\ \"very\".unusual"@strange.example.com (include non-letters character AND multiple at sign, the first one being double quoted)
}

func TestValidateHashedPassword(t *testing.T) {
	// NOTE: Python code used to generate hexidecimal sequences:
	//
	// random.choices(hexdigits.upper(), k=64).join()
	// "".join(random.choices(hexdigits.upper(), k=64))

	// invalid passwords
	assert.False(t, ValidatePasswordSha256("***#*2344"))
	assert.False(t, ValidatePasswordSha256("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFT"))
	assert.False(t, ValidatePasswordSha256("0123456789ABCDEFABCDEF"))                                                                                                           // too short
	assert.False(t, ValidatePasswordSha256("58EF14F177F26B2A12F16EA4FBFFFBC4FBB9E2ACC0EAFA8F11699E4EE324AA1FEB3A8A7A6A5BCADB3E24DB14882CB3D2CDD8E8DBBE02D1550DA9FDA9DED3E669")) // too long
	assert.False(t, ValidatePasswordSha256("C2161G0622EA40C42CFBCD7B9E1E57CC4CB26C520C722AE9BFB4BC2AF84BCDA5"))                                                                 // contains invalid character

	// valid hashed passwords
	assert.True(t, ValidatePasswordSha256(utilities.Sha256Checksum([]byte("some_bytes_here"))))
	assert.True(t, ValidatePasswordSha256(utilities.Sha256Checksum([]byte("4orYouWillDoe5erThings@"))))
	assert.True(t, ValidatePasswordSha256("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"))
	assert.True(t, ValidatePasswordSha256("0000000000000000000000000000000000000000000000000000000000000000"))
	assert.True(t, ValidatePasswordSha256("9BBAFAEEA0E53711CD6C123ADBDAC236957143E306ADC37B4A1C15E6B1CBD0A3"))
	assert.True(t, ValidatePasswordSha256("8274686282364632478430390097840070974254933463363963475973583772"))
}
