// Reference: https://github.com/google/re2/wiki/Syntax

package validation

import (
	"regexp"
	"strings"
)

// Password should contain at least 12 characters, but not exceed 32,
// at least one digit [0-9], one lower case letter [a-z], one upper case letter [A-Z],
// and one special character from a list: (@&$#%_)
func ValidatePassword(password string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9_@$#:&%]{12,32}$`)
	hasDigitsRe := regexp.MustCompile("[!^0-9]")
	hasLowerRe := regexp.MustCompile("[!^a-z]")
	hasUpperRe := regexp.MustCompile("[!^A-Z]")
	hasSymbolsRe := regexp.MustCompile(`[!^@|%|$|#|&]`)

	return re.MatchString(password) &&
		hasDigitsRe.MatchString(password) &&
		hasLowerRe.MatchString(password) &&
		hasUpperRe.MatchString(password) &&
		hasSymbolsRe.MatchString(password)
}

// Is used to validate participant's and channel's names.
// A name should be at least 8 characters long, but not exceed 32.
// And cannot start with a digit or an underscore  [0-9]|_.
// \w - [0-9A-Za-z_]
func ValidateName(name string) bool {
	re := regexp.MustCompile(`^[\w]{8,32}$`)
	beginWithRe := regexp.MustCompile(`^[a-zA-Z]`)

	return re.MatchString(name) &&
		beginWithRe.MatchString(name)
}

func ValidateEmail(email string) bool {
	// TODO(alx): Handle quoted email addresses?

	// Reference: https: //en.wikipedia.org/wiki/Email_address
	// Local-part
	// The local-part of the email address may be unquoted or may be enclosed in quotation marks.
	// If unquoted, it may use any of these ASCII characters:
	// 1. uppercase and lowercase Latin letters A to Z and a to z
	// 2. digits 0 to 9
	// 3. printable characters !#$%&'*+-/=?^_`{|}~
	// The maximum total length of the local-part of an email address is 64 octets.
	if strings.Count(email, "@") != 1 {
		return false
	}

	localPart, domainPart, _ := strings.Cut(email, "@")

	// dot cannot be the first or last character
	if strings.HasPrefix(localPart, ".") ||
		strings.HasSuffix(localPart, ".") {
		return false
	}

	for idx, c := range localPart {
		if c == '.' && (idx < len(localPart)-1) {
			// cannot have multiple .. consequtively
			if localPart[idx+1] == '.' {
				return false
			}
		}
	}

	// 4. dot ., provided that it is not the first or last character and provided also that it does not appear consecutively (e.g., John..Doe@example.com is not allowed).[7]
	// 	Domain
	// The domain name part of an email address has to conform to strict guidelines: it must match the requirements for a hostname, a list of dot-separated DNS labels, each label being limited to a length of 63 characters and consisting of:[7]: §2
	// 1. Uppercase and lowercase Latin letters A to Z and a to z;
	// 2. Digits 0 to 9, provided that top-level domain names are not all-numeric;
	// 3. Hyphen -, provided that it is not the first or last character.
	// 4. This rule is known as the LDH rule (letters, digits, hyphen). In addition, the domain may be an IP address literal, surrounded by square brackets [], such as jsmith@[192.168.2.1] or jsmith@[IPv6:2001:db8::1], although this is rarely seen except in email spam. Internationalized domain names (which are encoded to comply with the requirements for a hostname) allow for presentation of non-ASCII domains. In mail systems compliant with RFC 6531 and RFC 6532 an email address may be encoded as UTF-8, both a local-part as well as a domain name.
	// Comments are allowed in the domain as well as in the local-part; for example, john.smith@(comment)example.com and john.smith@example.com(comment) are equivalent to john.smith@example.com.

	localPartRe := regexp.MustCompile("^[0-9A-Za-z_.!#$\\%&'\\*\\+\\-/=\\?^`{\\|}~]{1,64}$")
	if !localPartRe.MatchString(localPart) {
		return false
	}

	if strings.HasPrefix(domainPart, "-") ||
		strings.HasSuffix(domainPart, "-") {
		return false
	}

	// NOTE(alx): Only match addresses which doesn't contain ip?
	// Then the regular expression would look like this:
	// `^[\w\-]+$`
	// And if we encounter '[]' symbols, we assume that it's an ip address.
	domainRe := regexp.MustCompile(`^[0-9A-Za-z\-\[\]:.]+$`)
	if !domainRe.MatchString(domainPart) {
		return false
	}

	if strings.HasPrefix(domainPart, "[") &&
		!strings.HasSuffix(domainPart, "]") {
		return false
	}

	return true
}

// A password hashed with sha256 algorithm should only contain hexdigits uppercase characters.
// [0-9A-F] with the length of 64
func ValidatePasswordSha256(passwordSha256 string) bool {
	re := regexp.MustCompile("^[0-9A-F]+$")
	return len(passwordSha256) == 64 &&
		re.MatchString(passwordSha256)
}
