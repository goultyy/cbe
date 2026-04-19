package cbe

// ID of market share
type ShareID GenericSecureID

// Key to authenticate share ownership
type ShareKey GenericSecureKey

// Direction of share on market
type ShareOutcomeDirection int

const (
	DIRECTION_YES ShareOutcomeDirection = iota // a share of 'yes' on current market
	DIRECTION_NO                               // no
)

// Share of market metadata
type Share struct {
	// Identifiers
	ShareID  ShareID
	MarketID MarketID // see notes on ShareOrder
	OrderID  OrderID  // order which created share

	// General data
	Direction ShareOutcomeDirection
	Price     USDCBaseAmount // total price of shares
	Quantity  int64          // number of shares

	// Authentication section
	Key ShareKey // 256-bit long string

	// Timestamps
	TimestampRequested Timestamp
	TimestampFulfilled Timestamp
}

// Order ID
type OrderID GenericSecureID

// Determines whether user wishes to trade on market price or to place limit order
type OrderType int

const (
	// A market order will be fulfilled at the best possible price, however still may not be filled
	ORDER_TYPE_MARKET OrderType = iota

	// A limit order will be fulfilled at the price specified by user or better (for buy, better means lower; for sell, better means higher)
	// the system will try the lowest amount first and work up for buy orders, and the highest amount first and work down for sell orders
	ORDER_TYPE_LIMIT
)

// Specifies how exchange should fulfill
type OrderForceType int

const (
	// (fill or kill) Take snapshot, fill entirely or cancel and do not fill.
	ORDER_FORCE_FOK OrderForceType = iota

	// (immediate or cancel) Take snapshot, fill as much then cancel the rest
	ORDER_FORCE_IOC

	// (good until cancel) Keep in order book, most generic one
	ORDER_FORCE_GTC
)

// Status of a buy/sell order
type OrderStatus int

const (
	ORDER_STATUS_PENDING OrderStatus = iota

	// Shares filled entirely, end of order lifecycle
	ORDER_STATUS_FILLED

	// Filled some of what was needed
	//
	// If ForceType == ORDER_FORCE_GTC, we keep trying to fill, if ForceType == ORDER_FORCE_IOC, we have
	// tried and nomore can be done. In both cases, we have some shares filled, but not all.
	ORDER_STATUS_PARTIALLY_FILLED

	// This refers to ForceType == ORDER_FORCE_FOK, not filled all, so cancelled, or the market has been cancelled,
	// so all orders which may remain are cancelled.
	ORDER_STATUS_CANCELLED
)

// Direction of order (buy/sell)
type OrderDirection int

const (
	ORDER_BUY  OrderDirection = iota // Buy
	ORDER_SELL                       // Sell
)

// Order to buy a share
type ShareOrder struct {
	// Identifier section
	OrderID OrderID
	// The market ID for a given share order, determined by the table for which this
	// order is present in
	MarketID MarketID

	Direction ShareOutcomeDirection
	Quantity  int64
	Status    OrderStatus
	BuySell   OrderDirection

	// Money information
	IPaymentID IPaymentID     // payout sent to this address
	BestPrice  USDCBaseAmount // Highest/lowest for current direction, relevant for ORDER_TYPE_LIMIT only
	Type       OrderType
	ForceType  OrderForceType

	TimestampCompleted Timestamp // Completion or cancellation
	TimestampRequested Timestamp
}
