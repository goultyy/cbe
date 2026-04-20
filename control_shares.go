package cbe

import "fmt"

// Create a share on the market
//
// Overrides Share.ShareID, Share.TimestampFulfilled and Share.Key
func CreateShare(share Share) (ShareID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return ShareID(""), err
	}

	if _, err := GetMarket(share.MarketID); err != nil {
		return ShareID(""), err
	}

	share.ShareID = ShareID(NewGenericSecureID())
	share.TimestampFulfilled = NewTimestamp()   // we can assume we create the share as soon as it's needed
	share.Key = ShareKey(NewGenericSecureKey()) // generate a new key for the share

	_, err = sql.Exec("INSERT INTO `"+string(share.MarketID)+"_shares` (ShareID, OrderID, Direction, Quantity, Price, `Key`, TimestampRequested, TimestampFulfilled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", share.ShareID, share.OrderID, share.Direction, share.Quantity, share.Price, share.Key, share.TimestampRequested, share.TimestampFulfilled)
	if err != nil {
		return ShareID(""), err
	}
	return share.ShareID, nil
}

// Get share by ID on a market
func (m Market) GetShare(share_id ShareID) (Share, error) {
	var data Share

	sql, err := ReturnSQLConnection()
	if err != nil {
		return Share{}, err
	}

	err = sql.QueryRow("SELECT ShareID, OrderID, Direction, Quantity, Price, Key, TimestampRequested, TimestampFulfilled FROM `"+string(m.MarketID)+"_shares` WHERE ShareID = ?", share_id).Scan(&data.ShareID, &data.OrderID, &data.Direction, &data.Quantity, &data.Price, &data.Key, &data.TimestampRequested, &data.TimestampFulfilled)
	if err != nil {
		return Share{}, err
	}

	return data, nil
}

// Get shares by order ID
func (m Market) GetSharesByOrderID(order_id OrderID) ([]Share, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query("SELECT ShareID, OrderID, Direction, Quantity, Price, Key, TimestampRequested, TimestampFulfilled FROM `"+string(m.MarketID)+"_shares` WHERE OrderID = ?", order_id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []Share
	for rows.Next() {
		var data Share
		err := rows.Scan(&data.ShareID, &data.OrderID, &data.Direction, &data.Quantity, &data.Price, &data.Key, &data.TimestampRequested, &data.TimestampFulfilled)
		if err != nil {
			return nil, err
		}
		shares = append(shares, data)
	}
	return shares, nil
}

// Get share order by ID
func (m Market) GetShareOrder(order_id OrderID) (ShareOrder, error) {
	var data ShareOrder
	sql, err := ReturnSQLConnection()
	if err != nil {
		return ShareOrder{}, err
	}

	err = sql.QueryRow("SELECT OrderID, Direction, Quantity, Status, IPaymentID, BestPrice, Type, ForceType, TimestampRequested, BuySell FROM `"+string(m.MarketID)+"_orders` WHERE OrderID = ?", order_id).Scan(&data.OrderID, &data.Direction, &data.Quantity, &data.Status, &data.IPaymentID, &data.BestPrice, &data.Type, &data.ForceType, &data.TimestampRequested, &data.BuySell)
	if err != nil {
		return ShareOrder{}, err
	}
	data.MarketID = m.MarketID
	return data, nil
}

// Get share order by their outcome direction (yes/no) and their buy/sell direction, orders by best price
func (m Market) GetShareOrdersByDirection(direction ShareOutcomeDirection, buy_sell OrderDirection) ([]ShareOrder, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	order_by := "ASC"
	if buy_sell == ORDER_BUY {
		order_by = "DESC"
	}

	// again, only return either pending or GTC partially filled.
	rows, err := sql.Query("SELECT OrderID, Direction, Quantity, Status, IPaymentID, BestPrice, Type, ForceType, TimestampRequested, BuySell FROM `"+string(m.MarketID)+"_orders` WHERE Direction = ? AND BuySell = ? AND ((Status = ?) OR (Status = ? AND ForceType = ?)) ORDER BY BestPrice "+order_by, direction, buy_sell, ORDER_STATUS_PENDING, ORDER_STATUS_PARTIALLY_FILLED, ORDER_FORCE_GTC)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []ShareOrder
	for rows.Next() {
		var data ShareOrder
		err := rows.Scan(&data.OrderID, &data.Direction, &data.Quantity, &data.Status, &data.IPaymentID, &data.BestPrice, &data.Type, &data.ForceType, &data.TimestampRequested, &data.BuySell)
		if err != nil {
			return nil, err
		}
		orders = append(orders, data)
	}
	return orders, nil
}

// Get share orders in a normalised view
//
// This is core logic for the program converting BUY 10 NO @ 0.2 => SELL 10 YES @ 0.8 and
// SELL 10 NO @ 0.2 => BUY 10 YES @ 0.8, this enforces the invariant that price_yes + price_no = 1,
// to ensure there are no gaps in the market.
//
// ShareOrder.Normalise does the same as this, but will not alter the ShareOutcomeDirection, this will
func (m Market) GetShareOrdersNormalised(buy_sell OrderDirection) ([]ShareOrder, error) {
	fmt.Printf("Getting direction buy = %t\n", buy_sell == ORDER_BUY)
	var orders []ShareOrder
	yes_orders, err := m.GetShareOrdersByDirection(DIRECTION_YES, buy_sell)
	if err != nil {
		return []ShareOrder{}, err
	}
	orders = append(orders, yes_orders...)

	// as sell no => buy no, we need to choose the inverse of what we originally wanted,
	// then convert it to the type we intended
	no_orders, err := m.GetShareOrdersByDirection(DIRECTION_NO, OrderDirection(1-int(buy_sell)))
	for _, v := range no_orders {
		if v.Type == ORDER_TYPE_LIMIT && v.Direction == DIRECTION_NO {
			// change limit order to enforce invariant for NO orders.
			v.BestPrice = ((USDC_BASE * 1) - v.BestPrice)
		}
		v.Direction = DIRECTION_YES // base direction
		v.BuySell = buy_sell        // safely do this, as we already flipped in GSOBD
		orders = append(orders, v)
	}
	return orders, nil
}

// Reduce quantity of share
//
// Used when somebody places a 'sell' order and it is processed
func (s Share) ReduceQuantity(new_quantity int64) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	if _, err := GetMarket(s.MarketID); err != nil {
		return err
	}

	// Just delete this record if quantity is reduced to 0, otherwise update it.
	if new_quantity == 0 {
		_, err = sql.Exec("DELETE FROM `"+string(s.MarketID)+"_shares` WHERE ShareID = ?", s.ShareID)
		if err != nil {
			return err
		}
		return nil
	}

	// Update it
	_, err = sql.Exec("UPDATE `"+string(s.MarketID)+"_shares` SET Quantity = ? WHERE ShareID = ?", new_quantity, s.ShareID)
	if err != nil {
		return err
	}
	return nil
}

// Quickly validate a key
func (s Share) ValidateKey(key ShareKey) bool {
	return s.Key == key
}

// Create new order on market
//
// Overrides ShareOrder.OrderID, ShareOrder.TimestampRequested, ShareOrder.TimestampCompleted and ShareOrder.Status
func CreateShareOrder(order ShareOrder) (OrderID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return OrderID(""), err
	}

	order.OrderID = OrderID(NewGenericSecureID())
	order.TimestampRequested = NewTimestamp()
	order.TimestampCompleted = 0
	order.Status = ORDER_STATUS_PENDING

	_, err = sql.Exec("INSERT INTO `"+string(order.MarketID)+"_orders` (OrderID, Direction, Quantity, Status, IPaymentID, BestPrice, Type, ForceType, TimestampCompleted, TimestampRequested, BuySell) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", order.OrderID, order.Direction, order.Quantity, order.Status, order.IPaymentID, order.BestPrice, order.Type, order.ForceType, order.TimestampCompleted, order.TimestampRequested, order.BuySell)
	if err != nil {
		return OrderID(""), err
	}
	return order.OrderID, nil
}

// Update orders status
//
// For a status indicating completion, that is status = ORDER_STATUS_PARTIALLY_FILLED and ForceType = ORDER_FORCE_TOC, status = ORDER_STATUS_FILLED
// or status = ORDER_STATUS_CANCELLED, we will make note of ShareOrder.TimestampCompleted.
func (s ShareOrder) UpdateStatus(new_status OrderStatus) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE `"+string(s.MarketID)+"_orders` SET Status = ? WHERE OrderID = ?", new_status, s.OrderID)

	if err != nil {
		return err
	}

	// if we have filled, or if we have partially filled but ForceType is IOC, we then mark as completed.
	if new_status == ORDER_STATUS_FILLED || (new_status == ORDER_STATUS_PARTIALLY_FILLED && s.ForceType == ORDER_FORCE_IOC) || new_status == ORDER_STATUS_CANCELLED {
		_, err = sql.Exec("UPDATE `"+string(s.MarketID)+"_orders` SET TimestampCompleted = ? WHERE OrderID = ?", NewTimestamp(), s.OrderID)
		if err != nil {
			return err
		}
	}

	return nil
}

// Reduce quantity of remaining shares in order to fulfill, needed for ForceType == ORDER_FORCE_GTC
// wherein we must keep trying to fulfill, or for ForceType == ORDER_FORCE_IOC where we fill as much then leave it
//
// This also sets the ShareOrder.TimestampCompleted for constraints outlined in ShareOrder.UpdateStatus
func (s ShareOrder) ReduceQuantity(new_quantity int64) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	if new_quantity == 0 {
		_, err := sql.Exec("UPDATE `"+string(s.MarketID)+"_orders` SET Status = ?, TimestampCompleted = ?, Quantity = ? WHERE OrderID = ?", ORDER_STATUS_FILLED, NewTimestamp(), new_quantity, s.OrderID)
		if err != nil {
			return err
		}
		return nil
	}

	_, err = sql.Exec("UPDATE `"+string(s.MarketID)+"_orders` SET Quantity = ? WHERE OrderID = ?", new_quantity, s.OrderID)
	if err != nil {
		return err
	}
	return nil
}

// Normalise a 'no' order (unchanged for 'yes')
//
// This will convert a 'no' order to the opposite order on the 'yes' market. Does not alter
// ShareOrderDirection.
func (s ShareOrder) Normalise() (ShareOrder, error) {
	if s.Direction == DIRECTION_YES {
		return s, nil
	}
	if s.Type == ORDER_TYPE_LIMIT {
		s.BestPrice = ((1 * USDC_BASE) - s.BestPrice)
	}
	s.BuySell = OrderDirection(1 - int(s.BuySell)) // sell => buy, buy => sell

	return s, nil
}
