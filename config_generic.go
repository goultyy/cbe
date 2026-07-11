package cbe

// Internal SQL config
type ConfigSQL struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// State of solana to use
type SolNetworkState int

const (
	SOL_NETWORK_STATE_DEVNET SolNetworkState = iota
	SOL_NETWORK_STATE_MAINNET
)

// Get current USDC mint address per state
func GetSolUSDCMint() string {
	if SOL_STATE == SOL_NETWORK_STATE_DEVNET {
		return SOL_USDC_DEVNET
	} else {
		return SOL_USDC_MAINNET
	}
}

// Get RPC address for solana
func GetSolRPCAddress() string {
	if SOL_STATE == SOL_NETWORK_STATE_DEVNET {
		return SOL_DEVNET_RPC
	} else {
		return SOL_MAINNET_RPC
	}
}
