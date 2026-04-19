package cbe

type IPaymentID GenericSecureID // internal payment ID

// Payment made to the treasury (central wallet) for market order
type Payment struct {
	// Identifiers
	IPaymentID          IPaymentID
	SolanaTransactionID string
	SourceUserID        string

	// All in USDC base amount (1e-6)
	Amount int64
	CBEFee int64 // Charged by cbe

	Timestamp Timestamp
}

type USDCBaseAmount int64 // Amount in USDC base units (1e-6)

func (a USDCBaseAmount) ToDecimal() float64 {
	return float64(a) / 1e6
}
