package session

import "regexp"

// https://github.com/google/re2/wiki/Syntax

var participantsNameRegexp = regexp.MustCompile("p([a-zA-z0-9_]+)")
var participantsPasswordRegexp = regexp.MustCompile("p()")

func validatePassword(password string) bool {
	length := len(password)
	return ((length >= 10) && (length <= 256)) &&
		participantsNameRegexp.MatchString(password)
}

func validateUsername(clientName string) bool {
	return false
}
