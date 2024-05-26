package session

import (
	"regexp"
)

func validatePassword(password string) bool {
	// Password should contain at least 12 characters,
	// at least one digit [0-9], one lower case letter [a-z], one upper case letter [A-Z],
	// and one special character from a list: (@&$#%_)
	re := regexp.MustCompile(`([a-zA-Z0-9_@$#_:&%%]{12,})`)
	hasDigitsRe := regexp.MustCompile(`([!^0-9])`)
	hasLowerRe := regexp.MustCompile(`([!^a-z])`)
	hasUpperRe := regexp.MustCompile(`([!^A-Z])`)
	hasSymbolsRe := regexp.MustCompile(`([!^@|%%|$|#|&])`)

	return re.MatchString(password) &&
		hasDigitsRe.MatchString(password) &&
		hasLowerRe.MatchString(password) &&
		hasUpperRe.MatchString(password) &&
		hasSymbolsRe.MatchString(password)
}

func validateUsername(clientName string) bool {
	return false
}
