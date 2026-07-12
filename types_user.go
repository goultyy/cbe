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
