package cbe

// Internal SQL config
type ConfigSQL struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// State of which USDC mint to use on Solana, testnet or mainnet
type SolUSDCState int

const (
	SOL_USDC_STATE_TESTNET SolUSDCState = iota
	SOL_USDC_STATE_MAINNET
)

// Get current USDC mint address per state
func GetSolUSDCMint() string {
	if SOL_USDC_STATE == SOL_USDC_STATE_TESTNET {
		return SOL_USDC_TESTNET
	} else {
		return SOL_USDC_MAINNET
	}
}
