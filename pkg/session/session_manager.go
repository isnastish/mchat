package session

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
