package session

// Use the, when the client rejoins the session,
// we should dump all the messages to a console, so that the data is persistent.

type SessionManager struct {
	sessions map[string]*Session
}

func (sm *SessionManager) HostSession(sessionName string) {
	if nil == sm.sessions {
		sm.sessions = make(map[string]*Session)
	}

	// should we pass session as an argument, or create it inplace?
	// sm.sessions[sessionName] = NewSession()
}

func (sm *SessionManager) ShutdownSession(sessionName string) bool {
	return true
}
