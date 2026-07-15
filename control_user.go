package cbe

import "fmt"

// Create a new notification for the user
func NewUserNotification(notification UserNotification) (GenericSecureID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return GenericSecureID(""), err
	}

	notification.NotificationID = NewGenericSecureID()
	_, err = sql.Exec(
		"INSERT INTO user_notifications (NotificationID, UserID, Source, Description, Priority, Timestamp) VALUES (?, ?, ?, ?, ?, ?)",
		notification.NotificationID,
		notification.UserID,
		notification.Source,
		notification.Description,
		int(notification.Priority),
		notification.Timestamp,
	)

	if err != nil {
		return GenericSecureID(""), err
	}

	return notification.NotificationID, nil
}

// Get all notifications for a user, ordered by timestamp descending, after a certain timestamp
func GetUserNotifications(user_id GenericSecureID, timestamp Timestamp) ([]UserNotification, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query(
		"SELECT NotificationID, UserID, Source, Description, Priority, Timestamp FROM user_notifications WHERE UserID = ? AND Timestamp > ? ORDER BY Timestamp DESC",
		user_id,
		timestamp,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []UserNotification

	for rows.Next() {
		var notification UserNotification
		err := rows.Scan(
			&notification.NotificationID,
			&notification.UserID,
			&notification.Source,
			&notification.Description,
			&notification.Priority,
			&notification.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		notifications = append(notifications, notification)
	}
	return notifications, nil
}

// Get specific notification
func GetUserNotificationByID(notification_id GenericSecureID) (UserNotification, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return UserNotification{}, err
	}

	row := sql.QueryRow(
		"SELECT NotificationID, UserID, Source, Description, Priority, Timestamp FROM user_notifications WHERE NotificationID = ?",
		notification_id,
	)

	var notification UserNotification
	err = row.Scan(
		&notification.NotificationID,
		&notification.UserID,
		&notification.Source,
		&notification.Description,
		&notification.Priority,
		&notification.Timestamp,
	)
	if err != nil {
		return UserNotification{}, err
	}
	return notification, nil
}

// Get notifications by priority
func GetUserNotificationsByPriority(user_id GenericSecureID, priority UserNotificationPriority, timestamp Timestamp) ([]UserNotification, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query(
		"SELECT NotificationID, UserID, Source, Description, Priority, Timestamp FROM user_notifications WHERE UserID = ? AND Priority = ? AND Timestamp > ? ORDER BY Timestamp DESC",
		user_id,
		int(priority),
		timestamp,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []UserNotification

	for rows.Next() {
		var notification UserNotification
		err := rows.Scan(
			&notification.NotificationID,
			&notification.UserID,
			&notification.Source,
			&notification.Description,
			&notification.Priority,
			&notification.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		notifications = append(notifications, notification)
	}
	return notifications, nil
}

// Create a new user
func NewUser() (User, error) {
	var new_user User
	// take nothing from the user, we do it all
	new_user.UserID = NewGenericSecureID()
	new_user.Key = NewGenericSecureKey()
	new_user.State = USER_STATE_ACTIVE
	new_user.Created = NewTimestamp()

	sql, err := ReturnSQLConnection()
	if err != nil {
		return User{}, err
	}

	_, err = sql.Exec("INSERT INTO users (UserID, UserKey, State, Created) VALUES (?, ?, ?, ?)", new_user.UserID, new_user.Key, new_user.State, new_user.Created)
	if err != nil {
		return User{}, err
	}

	return new_user, nil
}

// Get user by ID
func GetUserByID(user_id GenericSecureID) (User, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return User{}, err
	}

	row := sql.QueryRow(
		"SELECT UserID, UserKey, State, Created FROM users WHERE user_id = ?",
		user_id,
	)

	var user User
	err = row.Scan(
		&user.UserID,
		&user.Key,
		&user.State,
		&user.Created,
	)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

// Change user state
func (a User) ChangeUserState(new_state UserState) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE users SET State = ? WHERE UserID = ?", new_state, a.UserID)
	if err != nil {
		return err
	}

	return nil
}

// Allocate a share
func (a User) AllocateShare(share Share) error {
	if a.State != USER_STATE_ACTIVE {
		return fmt.Errorf("refusing to allocate to suspended user")
	}

	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	// Validate market & share from provided info
	market, err := GetMarket(share.MarketID)
	if err != nil {
		return fmt.Errorf("failed to find related market")
	}

	_, err = market.GetShare(share.ShareID)
	if err != nil {
		return fmt.Errorf("failed to find share")
	}

	_, err = sql.Exec("INSERT INTO user_shares (IUserID, ShareID, MarketID) VALUES (?, ?, ?)", a.UserID, share.ShareID, share.MarketID)
	if err != nil {
		return err
	}

	return nil
}

// Allocate a share order
func (a User) AllocateShareOrder(share_order ShareOrder) error {
	if a.State != USER_STATE_ACTIVE {
		return fmt.Errorf("refusing to allocate to suspended user")
	}

	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	// Validate market & share order from provided info
	market, err := GetMarket(share_order.MarketID)
	if err != nil {
		return fmt.Errorf("failed to find related market")
	}

	_, err = market.GetShareOrder(share_order.OrderID)
	if err != nil {
		return fmt.Errorf("failed to find share order")
	}

	_, err = sql.Exec("INSERT INTO user_share_orders (IUserID, OrderID, MarketID) VALUES (?, ?, ?)", a.UserID, share_order.OrderID, share_order.MarketID)
	if err != nil {
		return err
	}

	return nil
}

// Get shares related to a user with information from market.GetShare
// This is a big function with a lot of SQL queries..
func (a User) GetShares() ([]Share, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query(
		"SELECT ShareID, MarketID FROM user_shares WHERE IUserID = ?",
		a.UserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []Share

	for rows.Next() {
		var shareID ShareID
		var marketID MarketID

		err := rows.Scan(
			&shareID,
			&marketID,
		)
		if err != nil {
			return nil, err
		}

		market, err := GetMarket(marketID)
		if err != nil {
			return nil, fmt.Errorf("failed to find market for share %s: %w", shareID, err)
		}

		share, err := market.GetShare(shareID)
		if err != nil {
			return nil, fmt.Errorf("failed to get share %s from market %s: %w", shareID, marketID, err)
		}

		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return shares, nil
}

// Get share orders related to a user with information from market.GetShareOrder
func (a User) GetShareOrders() ([]ShareOrder, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query(
		"SELECT OrderID, MarketID FROM user_share_orders WHERE IUserID = ?",
		a.UserID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []ShareOrder

	for rows.Next() {
		var orderID OrderID
		var marketID MarketID

		err := rows.Scan(
			&orderID,
			&marketID,
		)
		if err != nil {
			return nil, err
		}

		market, err := GetMarket(marketID)
		if err != nil {
			return nil, fmt.Errorf("failed to find market for order %s: %w", orderID, err)
		}

		order, err := market.GetShareOrder(orderID)
		if err != nil {
			return nil, fmt.Errorf("failed to get order %s from market %s: %w", orderID, marketID, err)
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// Create a new user session
func NewSession(user_id GenericSecureID, browser_stamp string) (UserSessions, error) {
	user, err := GetUserByID(user_id)
	if err != nil {
		return UserSessions{}, err
	}

	if user.State != USER_STATE_ACTIVE {
		return UserSessions{}, fmt.Errorf("refusing to make session for inactive user")
	}

	sql, err := ReturnSQLConnection()
	if err != nil {
		return UserSessions{}, err
	}

	var ne UserSessions
	ne.Created = NewTimestamp()
	ne.SessionID = NewGenericSecureID()
	ne.SessionKey = NewGenericSecureKey()

	ne.UserID = user_id
	ne.Expiry = ne.Created + USER_SESSION_LENGTH
	ne.State = USER_SESSION_ACTIVE
	ne.BrowserStamp = browser_stamp

	_, err = sql.Exec("INSERT INTO user_sessions (SessionID, UserID, SessionKey, BrowserStamp, State, Created, Expiry) VALUES (?, ?, ?, ?, ?, ?, ?)", ne.SessionID, ne.UserID, ne.SessionKey, ne.BrowserStamp, ne.State, ne.Created, ne.Expiry)
	if err != nil {
		return UserSessions{}, err
	}

	return ne, nil
}

// Get session by ID
func GetSessionByID(session_id GenericSecureID) (UserSessions, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return UserSessions{}, err
	}

	var session UserSessions
	row := sql.QueryRow("SELECT SessionID, UserID, SessionKey, BrowserStamp, State, Created, Expiry FROM user_sessions WHERE SessionID = ?")
	err = row.Scan(
		&session.SessionID,
		&session.UserID,
		&session.SessionKey,
		&session.BrowserStamp,
		&session.State,
		&session.Created,
		&session.Expiry)

	return session, err
}

// Get session by SessionKey
func GetSessionByKey(session_key GenericSecureKey) (UserSessions, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return UserSessions{}, err
	}

	var session UserSessions
	row := sql.QueryRow("SELECT SessionID, UserID, SessionKey, BrowserStamp, State, Created, Expiry FROM user_sessions WHERE SessionKey = ?", session_key)
	err = row.Scan(
		&session.SessionID,
		&session.UserID,
		&session.SessionKey,
		&session.BrowserStamp,
		&session.State,
		&session.Created,
		&session.Expiry)

	return session, err
}

// Get all sessions for a user
func GetUserSessions(user_id GenericSecureID) ([]UserSessions, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query("SELECT SessionID, UserID, SessionKey, BrowserStamp, State, Created, Expiry FROM user_sessions WHERE UserID = ?", user_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []UserSessions
	for rows.Next() {
		var session UserSessions
		err := rows.Scan(
			&session.SessionID,
			&session.UserID,
			&session.SessionKey,
			&session.BrowserStamp,
			&session.State,
			&session.Created,
			&session.Expiry,
		)
		if err != nil {
			return nil, err
		}

		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

// Change the state of a session
func (a UserSessions) ChangeState(new_state UserSessionState) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE user_sessions SET State = ? WHERE SessionID = ?", new_state, a.SessionID)
	return err
}
