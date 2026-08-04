package cbe

import "fmt"

/*
	Todo: the codebase is split into two due to my late adaptation of the cbe package, which is slightly messy,
	because the user system was originally going to be separate, then I decided to add it into cbe, so we basically
	need to continously reference between the link tables for the relationships between users and share orders etc.

	this needs refactoring and a complete SQL schema redesign, but for now it works and is functional, so I will leave it as is.
*/

// Create a new notification for the user
func NewUserNotification(notification UserNotification) (GenericSecureID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return GenericSecureID(""), err
	}

	notification.NotificationID = NewGenericSecureID()
	notification.Timestamp = NewTimestamp()

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
		"SELECT UserID, UserKey, State, Created FROM users WHERE UserID = ?",
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

// Allocate a share, this can be empty - only requires ShareID & MarketID
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

// Allocate a share order, this can be an empty struct - it only requires MarketID & OrderID
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

// Get share by ID from User data
func (a User) GetShareByID(share_id ShareID) (Share, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return Share{}, err
	}

	row := sql.QueryRow(
		"SELECT MarketID FROM user_shares WHERE IUserID = ? AND ShareID = ?",
		a.UserID,
		share_id,
	)

	var marketID MarketID
	err = row.Scan(&marketID)
	if err != nil {
		return Share{}, err
	}

	market, err := GetMarket(marketID)
	if err != nil {
		return Share{}, fmt.Errorf("failed to find market for share %s: %w", share_id, err)
	}

	share, err := market.GetShare(share_id)
	if err != nil {
		return Share{}, fmt.Errorf("failed to get share %s from market %s: %w", share_id, marketID, err)
	}

	return share, nil
}

// Get share orders by ID from User data
func (a User) GetShareOrderByID(order_id OrderID) (ShareOrder, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return ShareOrder{}, err
	}

	row := sql.QueryRow(
		"SELECT MarketID FROM user_share_orders WHERE IUserID = ? AND OrderID = ?",
		a.UserID,
		order_id,
	)

	var marketID MarketID
	err = row.Scan(&marketID)
	if err != nil {
		return ShareOrder{}, err
	}

	market, err := GetMarket(marketID)
	if err != nil {
		return ShareOrder{}, fmt.Errorf("failed to find market for order %s: %w", order_id, err)
	}

	order, err := market.GetShareOrder(order_id)
	if err != nil {
		return ShareOrder{}, fmt.Errorf("failed to get order %s from market %s: %w", order_id, marketID, err)
	}

	return order, nil
}

// Reverse lookup which user initiated a share
func GetUserIDByShareID(share_id ShareID) (GenericSecureID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return GenericSecureID(""), err
	}

	row := sql.QueryRow(
		"SELECT IUserID FROM user_shares WHERE ShareID = ?",
		share_id,
	)

	var userID GenericSecureID
	err = row.Scan(&userID)
	if err != nil {
		return GenericSecureID(""), err
	}
	return userID, nil
}

// Reverse lookup which user initiated a share order
func GetUserIDByShareOrderID(order_id OrderID) (GenericSecureID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return GenericSecureID(""), err
	}

	row := sql.QueryRow(
		"SELECT IUserID FROM user_share_orders WHERE OrderID = ?",
		order_id,
	)

	var userID GenericSecureID
	err = row.Scan(&userID)

	if err != nil {
		return GenericSecureID(""), err
	}

	return userID, nil
}

// Get a MarketID by a ShareID, this is useful for reverse lookups when we only have a ShareID and need to find the MarketID
func GetMarketIDByShareID(share_id ShareID) (MarketID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return MarketID(""), err
	}

	row := sql.QueryRow(
		"SELECT MarketID FROM user_shares WHERE ShareID = ?",
		share_id,
	)

	var marketID MarketID
	err = row.Scan(&marketID)
	if err != nil {
		return MarketID(""), err
	}
	return marketID, nil
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
	row := sql.QueryRow("SELECT SessionID, UserID, SessionKey, BrowserStamp, State, Created, Expiry FROM user_sessions WHERE SessionID = ?", session_id)
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
