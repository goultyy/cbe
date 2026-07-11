package cbe

type IPaymentID GenericSecureID // internal payment ID

// Payment made to the treasury (central wallet) for market order
type Payment struct {
	// Identifiers
	IPaymentID          IPaymentID
	SolanaTransactionID string
	SourceUserID        GenericSecureID

	// All in USDC base amount (1e-6)
	Amount USDCBaseAmount
	CBEFee USDCBaseAmount // Charged by cbe

	SolanaFee SolanaBaseAmount // Fee charged by solana network

	Timestamp Timestamp
}

type SolanaWalletState int16 // State of the wallet

const (
	SOLANA_WALLET_STATE_ACTIVE SolanaWalletState = iota
	SOLANA_WALLET_STATE_DEACTIVATED
	SOLANA_WALLET_STATE_DISABLED
)

// Solana wallet for a user
type SolanaWallet struct {
	// Identifiers
	UserID    GenericSecureID
	IWalletID GenericSecureID

	// Wallet data
	PrivateKey string
	PublicKey  string

	// Metadata
	UserAddedDescription string         // 500 chars
	BalanceOnHold        USDCBaseAmount // Amount on hold for orders
	State                SolanaWalletState
	Created              Timestamp
}

type USDCBaseAmount int64 // Amount in USDC base units (1e-6)

var USDC_BASE USDCBaseAmount = 1000000 // Value of 1USDC in base units

func (a USDCBaseAmount) ToDecimal() float64 {
	return float64(a) / 1e6
}

type SolanaBaseAmount int64 // Amount in Solana base units (1e-9)

func (a SolanaBaseAmount) ToDecimal() float64 {
	return float64(a) / 1e9
}
