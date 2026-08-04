package cbe

type MarketID GenericUnsecureID

type MarketType int

const (
	MARKET_TYPE_SPORT MarketType = iota
	MARKET_TYPE_POLITICS
	MARKET_TYPE_ENTERTAINMENT
	MARKET_TYPE_FINANCE
)

type MarketState int

const (
	MARKET_STATE_OPEN      MarketState = iota // Available to trade on
	MARKET_STATE_CLOSED                       // Not available to trade on, but not yet resolved
	MARKET_STATE_CANCELLED                    // Slightly decieving, market cancelled, all shares paid out
)

// Market metadata
type Market struct {
	MarketID    MarketID
	Name        string // 100 characters
	Description string // 500 characters

	// Meta data
	Type  MarketType
	State MarketState

	// Timestamps
	TimestampCreated Timestamp
}

// Outcome direction statistics for a market
// Regularly 24-hour rolling average
type MarketDirectionStatistics struct {
	OutcomeDirection ShareOutcomeDirection

	TotalShares int64
	AskPrice    USDCBaseAmount
	BidPrice    USDCBaseAmount

	// Time period dependant
	TimeToClear float64 // Seconds until market clears
	TotalVolume int64

	TimestampLastTraded Timestamp
}

// Statistics for a market and outcome directions
type MarketStatistics struct {
	MarketID MarketID

	YesStatistics MarketDirectionStatistics
	NoStatistics  MarketDirectionStatistics
}

// Used to store changes to shares, until we process
type MarketMatchAdjustment struct {
	MarketID    MarketID
	OrderID     OrderID
	NewQuantity int64
}
