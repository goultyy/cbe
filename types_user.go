package cbe

type UserNotificationPriority int

const (
	NOTIF_PRIORITY_LOW    UserNotificationPriority = iota
	NOTIF_PRIORITY_MEDIUM                          // Used for share fulfilled etc
	NOTIF_PRIORITY_HIGH                            // Used for failed to fulfill share etc
)

// User notification
type UserNotification struct {
	// Identifiers
	NotificationID GenericSecureID
	UserID         GenericSecureID

	// Contents
	Source      string // 100 chars
	Description string // 500 chars

	// Meta data
	Priority  UserNotificationPriority
	Timestamp Timestamp
}

type UserState int

const (
	USER_STATE_ACTIVE    UserState = iota
	USER_STATE_SUSPENDED           // suspended users shouldn't be able to login
)

// User, hold as little data as possible
type User struct {
	// Identifiers
	UserID GenericSecureID

	// Meta data
	Key     GenericSecureKey // SQL this is UserKey
	State   UserState
	Created Timestamp
}

// State of a User Session
type UserSessionState int

const (
	USER_SESSION_ACTIVE        UserSessionState = iota
	USER_SESSION_TERMINATED                     // self terminated by a session owner
	USER_SESSION_EXPIRED                        // triggered by attempting to use an expired token
	USER_SESSION_SYSTERMINATED                  // terminated by the system for some reason
)

// Length of a user session
var USER_SESSION_LENGTH Timestamp = (60 * 60 * 24)

// Sessions (for any online based format i.e. APIs)
type UserSessions struct {
	// Identifiers
	SessionID GenericSecureID
	UserID    GenericSecureID

	// Data
	SessionKey   GenericSecureKey // generated (VARCHAR(5000))
	BrowserStamp string           // way to identify browser, could just be generated - doesn't really matter

	// Meta data
	State   UserSessionState
	Created Timestamp
	Expiry  Timestamp
}
