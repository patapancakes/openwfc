package gpcm

func kickPlayer(profileID uint32, reason string) {
	if session, exists := sessions[profileID]; exists {
		session.replyError(GPError{
			ErrorCode:   ErrConnectionClosed.ErrorCode,
			ErrorString: "The player was kicked from the server. Reason: " + reason,
			Fatal:       true,
		})
	}
}

func KickPlayer(profileID uint32, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	kickPlayer(profileID, reason)
}

func KickPlayerCustomMessage(profileID uint32, reason string) {
	mutex.Lock()
	defer mutex.Unlock()

	if session, exists := sessions[profileID]; exists {
		session.replyError(GPError{
			ErrorCode:   ErrConnectionClosed.ErrorCode,
			ErrorString: "The player was kicked from the server. Reason: " + reason,
			Fatal:       true,
		})
	}
}
