package cbe

// Get market by ID
func GetMarket(market_id MarketID) (Market, error) {
	var data Market

	sql, err := ReturnSQLConnection()
	if err != nil {
		return Market{}, err
	}

	err = sql.QueryRow("SELECT MarketID, Name, Description, Type, TimestampCreated FROM markets WHERE MarketID = ?", market_id).Scan(&data.MarketID, &data.Name, &data.Description, &data.Type, &data.TimestampCreated)
	if err != nil {
		return Market{}, err
	}

	return data, nil
}

// Create a market with given metadata. Returns market ID or error if failed.
// This will make shares and orders table too
func CreateMarket(m Market) (MarketID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return MarketID(""), err
	}

	// do not preserve from user
	m.MarketID = MarketID(NewGenericUnsecureID())
	m.TimestampCreated = NewTimestamp()
	m.State = MARKET_STATE_CLOSED // do not open the market yet

	_, err = sql.Exec("INSERT INTO markets (MarketID, Name, Description, Type, State, TimestampCreated) VALUES (?, ?, ?, ?, ?, ?)", m.MarketID, m.Name, m.Description, m.Type, m.State, m.TimestampCreated)
	if err != nil {
		return MarketID(""), err
	}

	// now, we have to create the order book & share book for the market.
	query := "CREATE TABLE `" + string(m.MarketID) + "_orders` (`OrderID` VARCHAR(64) NOT NULL,`Direction` INT NULL,`Quantity` INT NULL,`Status` INT NULL,`BuySell` INT NULL,`IPaymentID` VARCHAR(64) NULL,`BestPrice` BIGINT NULL,\n\t`Type` INT NULL,\n\t`ForceType` INT NULL,\n\t`TimestampCompleted` BIGINT NULL,\n\t`TimestampRequested` BIGINT NULL,\n\tPRIMARY KEY (`OrderID`));\n"

	_, err = sql.Exec(query)
	if err != nil {
		return MarketID(""), err
	}

	query = "CREATE TABLE `" + string(m.MarketID) + "_shares` (`ShareID` VARCHAR(64) NOT NULL,`OrderID` VARCHAR(64) NOT NULL,`Direction` INT NULL,`Quantity` INT NULL,`Price` BIGINT NOT NULL,`Key` VARCHAR(256) NULL,`TimestampRequested` BIGINT NOT NULL,`TimestampFulfilled` BIGINT NULL,\n\tPRIMARY KEY (`ShareID`));\n"
	_, err = sql.Exec(query)
	if err != nil {
		return MarketID(""), err
	}

	return m.MarketID, nil
}

// Change the state of a market
func (m Market) ChangeState(new_state MarketState) error {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}
	_, err = sql.Exec("UPDATE markets SET State = ? WHERE MarketID = ?", new_state, m.MarketID)

	if err != nil {
		return err
	}

	if new_state == MARKET_STATE_CANCELLED {
		// we must now cancel all open orders, these are ForceType == ORDER_FORCE_GTC
		// we leave any partially filled orders
		_, err = sql.Exec("UPDATE `"+string(m.MarketID)+"_orders` SET Status = ? WHERE Status = ?", ORDER_STATUS_CANCELLED, ORDER_STATUS_PENDING)
		if err != nil {
			return err
		}
	}

	return nil
}

// Get market statistics by direction (yes/no)
// this will generate an error on no orders which you can safely ignore and treat as zero vals.
func (m Market) GetMarketStatisticsDirection(direction ShareOutcomeDirection) (MarketDirectionStatistics, error) {
	var stats MarketDirectionStatistics
	stats.OutcomeDirection = direction

	sql, err := ReturnSQLConnection()
	if err != nil {
		return MarketDirectionStatistics{}, err
	}

	// find volume and total shares
	err = sql.QueryRow("SELECT COALESCE(SUM(Quantity), 0), COALESCE(SUM(Quantity * BestPrice), 0) FROM `"+string(m.MarketID)+"_orders` WHERE Direction = ? AND (`TimestampRequested` < ? AND `TimestampRequested` > ?)", direction, NewTimestamp(), NewTimestamp()-86400).Scan(&stats.TotalShares, &stats.TotalVolume)
	if err != nil {
		return MarketDirectionStatistics{}, err
	}

	// these two we want any orders that are pending, or GTC and partially filled
	// they will also only look for limit orders as these actually give a price

	// find ask price, this is the lowest sell order
	err = sql.QueryRow("SELECT COALESCE(BestPrice, 0) FROM `"+string(m.MarketID)+"_orders` WHERE ((`Status` = ?) OR (`Status` = ?  AND `ForceType` = ?)) AND Direction = ? AND BuySell = ? AND `Type` = ? ORDER BY BestPrice ASC LIMIT 1", ORDER_STATUS_PENDING, ORDER_STATUS_PARTIALLY_FILLED, ORDER_FORCE_GTC, direction, ORDER_SELL, ORDER_TYPE_LIMIT).Scan(&stats.AskPrice)
	if err != nil {
		return MarketDirectionStatistics{}, err
	}

	// find bid price, this is the highest buy order
	err = sql.QueryRow("SELECT COALESCE(BestPrice, 0) FROM `"+string(m.MarketID)+"_orders` WHERE ((`Status` = ?) OR (`Status` = ?  AND `ForceType` = ?)) AND Direction = ? AND BuySell = ? AND `Type` = ? ORDER BY BestPrice DESC LIMIT 1", ORDER_STATUS_PENDING, ORDER_STATUS_PARTIALLY_FILLED, ORDER_FORCE_GTC, direction, ORDER_BUY, ORDER_TYPE_LIMIT).Scan(&stats.BidPrice)
	if err != nil {
		return MarketDirectionStatistics{}, err
	}

	// find time to clear (Completed - Requested), we look where (Status = FILELD) or (Status = PARTIALLY_FILLED and ForceType = ORDER_FORCE_IOC)
	err = sql.QueryRow("SELECT COALESCE(AVG(TimestampCompleted - TimestampRequested), 0) FROM `"+string(m.MarketID)+"_orders` WHERE ((`Status` = ?) OR (`Status` = ?  AND `ForceType` = ?)) AND `Direction` = ? AND (`TimestampRequested` < ? AND `TimestampRequested` > ?)", ORDER_STATUS_FILLED, ORDER_STATUS_PARTIALLY_FILLED, ORDER_FORCE_IOC, direction, NewTimestamp(), NewTimestamp()-86400).Scan(&stats.TimeToClear)
	if err != nil {
		return MarketDirectionStatistics{}, err
	}

	return stats, nil
}

// Get market statistics for both directions
func (m Market) GetMarketStatistics() (MarketStatistics, error) {
	var stats MarketStatistics
	var err error

	stats.MarketID = m.MarketID

	stats.YesStatistics, err = m.GetMarketStatisticsDirection(DIRECTION_YES)
	stats.NoStatistics, err = m.GetMarketStatisticsDirection(DIRECTION_NO)

	_ = err
	return stats, nil
}
