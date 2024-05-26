package session

import (
	"regexp"
	"strings"
)

// Reference: https://github.com/google/re2/wiki/Syntax

func validatePassword(password string) bool {
	// Password should contain at least 12 characters, but not exceed 32,
	// at least one digit [0-9], one lower case letter [a-z], one upper case letter [A-Z],
	// and one special character from a list: (@&$#%_)
	re := regexp.MustCompile(`^[a-zA-Z0-9_@$#_:&%]{12,32}$`)
	hasDigitsRe := regexp.MustCompile("([!^0-9])")
	hasLowerRe := regexp.MustCompile("([!^a-z])")
	hasUpperRe := regexp.MustCompile("([!^A-Z])")
	hasSymbolsRe := regexp.MustCompile(`([!^@|%|$|#|&])`)

	return re.MatchString(password) &&
		hasDigitsRe.MatchString(password) &&
		hasLowerRe.MatchString(password) &&
		hasUpperRe.MatchString(password) &&
		hasSymbolsRe.MatchString(password)
}

func validateName(name string) bool {
	// Is used to validate participant's and channel's names.
	// A name should be at least 8 characters long, but not exceed 32.
	// And cannot start with a digit or an underscore  [0-9]|_.
	// \w - [0-9A-Za-z_]
	re := regexp.MustCompile(`^[\w]{8,32}$`)
	beginWithRe := regexp.MustCompile(`^[a-zA-Z]`)

	return re.MatchString(name) &&
		beginWithRe.MatchString(name)
}

func validateEmailAddress(emailAddress string) bool {
	// Local-part
	// The local-part of the email address may be unquoted or may be enclosed in quotation marks.

	// If unquoted, it may use any of these ASCII characters:
	// 1. uppercase and lowercase Latin letters A to Z and a to z
	// 2. digits 0 to 9
	// 3. printable characters !#$%&'*+-/=?^_`{|}~
	// 4. dot ., provided that it is not the first or last character and provided also that it does not appear consecutively (e.g., John..Doe@example.com is not allowed).[7]

	// See for reference: https: //en.wikipedia.org/wiki/Email_address

	// 	Domain
	// The domain name part of an email address has to conform to strict guidelines: it must match the requirements for a hostname, a list of dot-separated DNS labels, each label being limited to a length of 63 characters and consisting of:[7]: §2
	// 1. Uppercase and lowercase Latin letters A to Z and a to z;
	// 2. Digits 0 to 9, provided that top-level domain names are not all-numeric;
	// 3. Hyphen -, provided that it is not the first or last character.
	// 4. This rule is known as the LDH rule (letters, digits, hyphen). In addition, the domain may be an IP address literal, surrounded by square brackets [], such as jsmith@[192.168.2.1] or jsmith@[IPv6:2001:db8::1], although this is rarely seen except in email spam. Internationalized domain names (which are encoded to comply with the requirements for a hostname) allow for presentation of non-ASCII domains. In mail systems compliant with RFC 6531 and RFC 6532 an email address may be encoded as UTF-8, both a local-part as well as a domain name.

	// Comments are allowed in the domain as well as in the local-part; for example, john.smith@(comment)example.com and john.smith@example.com(comment) are equivalent to john.smith@example.com.
	localPart, domainPart, found := strings.Cut(emailAddress, "@")
	if !found {
		return false
	}

	_ = localPart
	_ = domainPart

	return true
}
