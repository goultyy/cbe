package cbe

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// There are two types of transaction, I and H, I transactions are between cbe wallets,
// H ones leave cbe wallets to an external one. (I=Internal, H=Handoff)
//
// Functions exposed to user space (or even outside the module) should ensure that
// they check:
// 1. The source wallet is active
// 2. Destination is active
// 3. Source wallet can afford (i.e. relative to funds on holds)
//
// TODO Refactor: Most I transaction functions are simply slight changes of H ones also add sending to be related to a wallet (a Wallet).Send...

// Create new payment
func CreatePayment(payment Payment) (IPaymentID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return IPaymentID(""), err
	}

	payment.IPaymentID = IPaymentID(NewGenericSecureID())
	payment.Timestamp = NewTimestamp()

	_, err = sql.Exec("INSERT INTO payments (IPaymentID, SolanaTransactionID, SourceUserID, Amount, SolanaAmount, USDCAmount, CBEFee, SolanaFee, Timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", payment.IPaymentID, payment.SolanaTransactionID, string(payment.SourceUserID), payment.Amount, payment.SolanaAmount, payment.USDCAmount, payment.CBEFee, payment.SolanaFee, payment.Timestamp)
	if err != nil {
		return IPaymentID(""), err
	}
	return payment.IPaymentID, nil
}

// Get payment information
func GetPaymentByID(ipayment_id GenericSecureID) (Payment, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return Payment{}, err
	}

	row := sql.QueryRow("SELECT IPaymentID, SolanaTransactionID, SourceUserID, Amount, SolanaAmount, USDCAmount, CBEFee, SolanaFee, Timestamp FROM payments WHERE IPaymentID = ?", string(ipayment_id))

	var payment Payment
	err = row.Scan(&payment.IPaymentID, &payment.SolanaTransactionID, &payment.SourceUserID, &payment.Amount, &payment.SolanaAmount, &payment.USDCAmount, &payment.CBEFee, &payment.SolanaFee, &payment.Timestamp)
	if err != nil {
		return Payment{}, err
	}

	return payment, nil
}

// Get all payments
func GetAllPayments(userid GenericSecureID, timestamp_start Timestamp, timestamp_end Timestamp) ([]Payment, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query("SELECT IPaymentID, SolanaTransactionID, SourceUserID, Amount, SolanaAmount, USDCAmount, CBEFee, SolanaFee, Timestamp FROM payments WHERE SourceUserID = ? AND Timestamp >= ? AND Timestamp <= ? ORDER BY Timestamp DESC", userid, timestamp_start, timestamp_end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []Payment
	for rows.Next() {
		var p Payment
		err := rows.Scan(&p.IPaymentID, &p.SolanaTransactionID, &p.SourceUserID, &p.Amount, &p.SolanaAmount, &p.USDCAmount, &p.CBEFee, &p.SolanaFee, &p.Timestamp)
		if err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return payments, nil
}

// Can a user afford a transaction with the amount on hold
func (a SolanaWallet) CanAffordUSDC(amount USDCBaseAmount) bool {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return false
	}

	bal, err := GetSolanaWalletUSDCBalance(a.IWalletID)
	if err != nil {
		return false
	}

	if a.BalanceOnHold+amount > bal {
		return false
	}

	return true
}

// Can a afford SOL transaction
func (a SolanaWallet) CanAffordSOL(amount SolanaBaseAmount) bool {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return false
	}

	bal, err := GetSolanaWalletBalance(a.IWalletID)
	if err != nil {
		return false
	}

	if a.BalanceOnHoldSOL+amount > bal {
		return false
	}

	return true
}

// Place funds on hold, notification issued in new thread
func (a SolanaWallet) PlaceOnHoldUSDC(amount USDCBaseAmount) error {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return fmt.Errorf("wallet is not active")
	}

	bal, err := GetSolanaWalletUSDCBalance(a.IWalletID)
	if err != nil {
		return err
	}

	if bal < amount {
		return fmt.Errorf("wallet cannot afford amount")
	}

	a.BalanceOnHold += amount
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE user_wallets SET BalanceOnHold = ? WHERE IWalletID = ?", a.BalanceOnHold, a.IWalletID)
	if err != nil {
		return err
	}

	go NewUserNotification(UserNotification{
		UserID:      a.UserID,
		Source:      "System placed hold on wallet",
		Description: fmt.Sprintf("System placed hold on wallet %s by %d USDC base units, new hold is %d USDC base units", a.IWalletID, amount, a.BalanceOnHold),
	})

	return nil
}

// Place SOL amount on hold
func (a SolanaWallet) PlaceOnHoldSOL(amount SolanaBaseAmount) error {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return fmt.Errorf("wallet is not active")
	}

	bal, err := GetSolanaWalletBalance(a.IWalletID)
	if err != nil {
		return err
	}

	if bal < amount {
		return fmt.Errorf("wallet cannot afford amount")
	}

	a.BalanceOnHoldSOL += amount
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE user_wallets SET BalanceOnHoldSOL = ? WHERE IWalletID = ?", a.BalanceOnHold, a.IWalletID)
	if err != nil {
		return err
	}

	go NewUserNotification(UserNotification{
		UserID:      a.UserID,
		Source:      "System placed hold on wallet (SOL)",
		Description: fmt.Sprintf("System placed hold on wallet %s by %d SOL base units, new hold is %d SOL base units", a.IWalletID, amount, a.BalanceOnHoldSOL),
	})

	return nil
}

// Reduce on hold (a.BalanceOnHold = a.BalanceOnHold - amount), notification issued in new thread
func (a SolanaWallet) ReduceHold(amount USDCBaseAmount) error {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return fmt.Errorf("wallet is not active")
	}

	if a.BalanceOnHold < amount {
		return fmt.Errorf("wallet cannot reduce hold by amount")
	}

	a.BalanceOnHold -= amount
	sql, err := ReturnSQLConnection()
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE user_wallets SET BalanceOnHold = ? WHERE IWalletID = ?", a.BalanceOnHold, a.IWalletID)
	if err != nil {
		return err
	}

	go NewUserNotification(UserNotification{
		UserID:      a.UserID,
		Source:      "System reduced hold on wallet",
		Description: fmt.Sprintf("System reduced hold on wallet %s by %d USDC base units, new hold is %d USDC base units", a.IWalletID, amount, a.BalanceOnHold),
	})

	return nil
}

// Generate token account
func (a SolanaWallet) GenerateATAAccount() error {
	if a.State != SOLANA_WALLET_STATE_ACTIVE {
		return fmt.Errorf("wallet not active")
	}

	owner := solana.MustPublicKeyFromBase58(a.PublicKey)
	mint := solana.MustPublicKeyFromBase58(GetSolUSDCMint())
	payer := owner
	priv := solana.MustPrivateKeyFromBase58(a.PrivateKey)
	client := rpc.New(GetSolRPCAddress())

	ix := associatedtokenaccount.NewCreateInstruction(
		payer,
		owner,
		mint,
	).Build()

	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return err
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(payer),
	)
	if err != nil {
		return err
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer) {
			return &priv
		}
		return nil
	})
	if err != nil {
		return err
	}

	_, err = client.SendTransaction(context.Background(), tx)
	return err
}

// Generate a new Solana wallet for a user and store it in the database. Returns the wallet or error if failed.
func GenerateSolanaWallet(user_id GenericSecureID, description string) (SolanaWallet, error) {
	wallet := solana.NewWallet()

	solana_wallet := SolanaWallet{
		UserID:               user_id,
		IWalletID:            GenericSecureID(NewGenericSecureID()),
		PrivateKey:           wallet.PrivateKey.String(),
		PublicKey:            wallet.PublicKey().String(),
		Created:              NewTimestamp(),
		State:                SOLANA_WALLET_STATE_ACTIVE,
		UserAddedDescription: description,
	}

	sql, err := ReturnSQLConnection()
	if err != nil {
		return SolanaWallet{}, err
	}

	_, err = sql.Exec("INSERT INTO user_wallets (UserID, IWalletID, PrivateKey, PublicKey, State, Created, UserAddedDescription, BalanceOnHold, BalanceOnHoldSOL) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", solana_wallet.UserID, solana_wallet.IWalletID, solana_wallet.PrivateKey, solana_wallet.PublicKey, solana_wallet.State, solana_wallet.Created, solana_wallet.UserAddedDescription, solana_wallet.BalanceOnHold, solana_wallet.BalanceOnHoldSOL)
	if err != nil {
		return SolanaWallet{}, err
	}

	return solana_wallet, nil
}

// Get solana wallets by user account
func GetSolanaWallets(user_id GenericSecureID) ([]SolanaWallet, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return nil, err
	}

	rows, err := sql.Query("SELECT UserID, IWalletID, PrivateKey, PublicKey, State, Created, UserAddedDescription, BalanceOnHold, BalanceOnHoldSOL FROM user_wallets WHERE UserID = ?", user_id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var wallets []SolanaWallet
	for rows.Next() {
		var wallet SolanaWallet
		err := rows.Scan(&wallet.UserID, &wallet.IWalletID, &wallet.PrivateKey, &wallet.PublicKey, &wallet.State, &wallet.Created, &wallet.UserAddedDescription, &wallet.BalanceOnHold, &wallet.BalanceOnHoldSOL)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}

// Get a wallet by it's ID
func GetSolanaWalletByID(wallet_id GenericSecureID) (SolanaWallet, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return SolanaWallet{}, err
	}

	row := sql.QueryRow("SELECT UserID, IWalletID, PrivateKey, PublicKey, State, Created, UserAddedDescription, BalanceOnHold, BalanceOnHoldSOL FROM user_wallets WHERE IWalletID = ?", wallet_id)

	var wallet SolanaWallet
	err = row.Scan(&wallet.UserID, &wallet.IWalletID, &wallet.PrivateKey, &wallet.PublicKey, &wallet.State, &wallet.Created, &wallet.UserAddedDescription, &wallet.BalanceOnHold, &wallet.BalanceOnHoldSOL)

	if err != nil {
		return SolanaWallet{}, err
	}

	return wallet, nil
}

// Get USDC balance of a solana wallet
func GetSolanaWalletUSDCBalance(wallet_id GenericSecureID) (USDCBaseAmount, error) {
	client := rpc.New(GetSolRPCAddress())
	wallet_internal, err := GetSolanaWalletByID(wallet_id)

	if err != nil {
		return USDCBaseAmount(0), err
	}

	if wallet_internal.State != SOLANA_WALLET_STATE_ACTIVE {
		return USDCBaseAmount(0), fmt.Errorf("will not fetch inactive wallet details")
	}

	wallet := solana.MustPublicKeyFromBase58(wallet_internal.PublicKey)
	ata, _, err := solana.FindAssociatedTokenAddress(wallet, solana.MustPublicKeyFromBase58(GetSolUSDCMint()))

	if err != nil {
		return USDCBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	balance, err := client.GetTokenAccountBalance(context.Background(), ata, rpc.CommitmentFinalized)
	if err != nil {
		return USDCBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	amount, err := strconv.Atoi(balance.Value.Amount)
	if err != nil {
		return USDCBaseAmount(0), fmt.Errorf("integer conversion")
	}

	return USDCBaseAmount(amount), nil
}

// Get SOL balance of a solana wallet
func GetSolanaWalletBalance(wallet_id GenericSecureID) (SolanaBaseAmount, error) {
	client := rpc.New(GetSolRPCAddress())
	wallet_internal, err := GetSolanaWalletByID(wallet_id)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	if wallet_internal.State != SOLANA_WALLET_STATE_ACTIVE {
		return SolanaBaseAmount(0), fmt.Errorf("will not fetch inactive wallet details")
	}

	wallet := solana.MustPublicKeyFromBase58(wallet_internal.PublicKey)
	balance, err := client.GetBalance(context.Background(), wallet, rpc.CommitmentFinalized)
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	return SolanaBaseAmount(int64(balance.Value)), nil
}

// Get the ATA for a wallet and mint
func GetSolanaWalletATA(wallet string, mint string) (string, error) {
	wallet_pubkey, err := solana.PublicKeyFromBase58(wallet)
	if err != nil {
		return "", fmt.Errorf("solana (wallet) - %s", err.Error())
	}
	mint_pubkey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("solana (mint) - %s", err.Error())
	}
	ata, _, err := solana.FindAssociatedTokenAddress(wallet_pubkey, mint_pubkey)
	if err != nil {
		return "", fmt.Errorf("solana (ata) - %s", err.Error())
	}

	account, err := rpc.New(GetSolRPCAddress()).GetAccountInfo(
		context.Background(), ata,
	)
	if err != nil {
		return "", fmt.Errorf("solana (ata lookup) - %s", err.Error())
	}
	if account == nil || account.Value == nil {
		return "", fmt.Errorf("solana (ata) - account does not exist")
	}
	if account.Value.Owner != token.ProgramID {
		return "", fmt.Errorf("solana (ata) - account is not token-owned")
	}

	return ata.String(), nil
}

// Estimate fee for a generic solana transaction
func estimateFeeSOLTransaction(source string, dest string, amount SolanaBaseAmount) (SolanaBaseAmount, error) {
	client := rpc.New(GetSolRPCAddress())
	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	sourcePubkey := solana.MustPublicKeyFromBase58(source)
	destPubkey := solana.MustPublicKeyFromBase58(dest)
	ix := system.NewTransferInstruction(uint64(amount), sourcePubkey, destPubkey).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(sourcePubkey),
	)
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	bin, err := tx.Message.MarshalBinary()
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("%s", err.Error())
	}

	msgBase64 := base64.StdEncoding.EncodeToString(bin)
	fee, err := client.GetFeeForMessage(context.Background(), msgBase64, rpc.CommitmentFinalized)
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	// int64(*..) is risky as uint64 holds more, however we assume solana fees are <<<
	return SolanaBaseAmount(int64(*fee.Value)), nil
}

// Estimate fee for an 'H'-type solana transaction
func EstimateFeeSOLHTransaction(source GenericSecureID, dest string, amount SolanaBaseAmount) (SolanaBaseAmount, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return SolanaBaseAmount(0), fmt.Errorf("will not estimate fee for inactive wallet")
	}

	return estimateFeeSOLTransaction(source_wallet.PublicKey, dest, amount)
}

// Estimate the fee for a SOL transaction between two cbe controlled wallets
func EstimateFeeSOLITransaction(source GenericSecureID, dest GenericSecureID, amount SolanaBaseAmount) (SolanaBaseAmount, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	dest_wallet, err := GetSolanaWalletByID(dest)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE || dest_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return SolanaBaseAmount(0), fmt.Errorf("will not estimate fee for inactive wallet")
	}

	return estimateFeeSOLTransaction(source_wallet.PublicKey, dest_wallet.PublicKey, amount)
}

// (Internal) new solana transaction
func newSOLTransaction(source string, private_key string, destination string, amount SolanaBaseAmount) (string, error) {
	client := rpc.New(GetSolRPCAddress())
	priv := solana.MustPrivateKeyFromBase58(private_key)
	src := solana.MustPublicKeyFromBase58(source)
	dest := solana.MustPublicKeyFromBase58(destination)

	ix := system.NewTransferInstruction(uint64(amount), src, dest).Build()
	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(src),
	)
	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(src) {
			return &priv
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	sig, err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}
	return sig.String(), nil
}

// New SOL internal transaction
func NewSOLITransaction(source GenericSecureID, dest GenericSecureID, amount SolanaBaseAmount) (IPaymentID, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return "", err
	}

	if !source_wallet.CanAffordSOL(amount) {
		return "", fmt.Errorf("source wallet cannot afford transaction")
	}

	dest_wallet, err := GetSolanaWalletByID(dest)
	if err != nil {
		return "", err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE || dest_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return "", fmt.Errorf("will not send transaction for inactive wallet")
	}

	txid, err := newSOLTransaction(source_wallet.PublicKey, source_wallet.PrivateKey, dest_wallet.PublicKey, amount)

	if err != nil {
		return "", err
	}

	ipayid := NewGenericSecureID()
	timestamp := NewTimestamp()
	fee, err := EstimateFeeSOLITransaction(source, dest, amount) // estimate it first, find it later/don't rely on this field
	if err != nil {
		return "", err
	}

	payment := Payment{
		IPaymentID:          IPaymentID(ipayid),
		SolanaTransactionID: txid,
		SourceUserID:        source_wallet.UserID,
		USDCAmount:          0,
		SolanaAmount:        amount,
		CBEFee:              0,
		SolanaFee:           fee,
		Timestamp:           timestamp,
		Amount:              0,
	}

	id, err := CreatePayment(payment)
	if err != nil {
		return "", err
	}

	return id, nil
}

// New SOL leaving controlled cbe controlled wallet
func NewSOLHTransaction(source GenericSecureID, dest string, amount SolanaBaseAmount) (IPaymentID, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return "", err
	}

	if !source_wallet.CanAffordSOL(amount) {
		return "", fmt.Errorf("source wallet cannot afford transaction")
	}

	dest_wallet, err := GetSolanaWalletATA(dest, GetSolUSDCMint())
	if err != nil {
		return "", err
	}

	txid, err := newSOLTransaction(source_wallet.PublicKey, source_wallet.PrivateKey, dest_wallet, amount)
	if err != nil {
		return "", err
	}

	ipayid := NewGenericSecureID()
	timestamp := NewTimestamp()
	fee, err := EstimateFeeSOLHTransaction(source, dest, amount)

	if err != nil {
		return "", err
	}

	payment := Payment{
		IPaymentID:          IPaymentID(ipayid),
		SolanaTransactionID: txid,
		SourceUserID:        source_wallet.UserID,
		USDCAmount:          0,
		SolanaAmount:        amount,
		CBEFee:              0,
		SolanaFee:           fee,
		Timestamp:           timestamp,
	}

	id, err := CreatePayment(payment)
	if err != nil {
		return "", err
	}
	return id, nil
}

// (Internal) Estimate the fee for a transaction between two wallets)
func estimateFeeUSDCTransaction(source string, dest string, amount USDCBaseAmount) (SolanaBaseAmount, error) {
	client := rpc.New(GetSolRPCAddress())
	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)

	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	source_ata, err := GetSolanaWalletATA(source, GetSolUSDCMint())
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana (ata) - %s", err.Error())
	}

	dest_ata, err := GetSolanaWalletATA(dest, GetSolUSDCMint())
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana (ata) - %s", err.Error())
	}

	ix := token.NewTransferInstruction(
		uint64(amount),
		solana.MustPublicKeyFromBase58(source_ata),
		solana.MustPublicKeyFromBase58(dest_ata),
		solana.MustPublicKeyFromBase58(source),
		nil,
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(solana.MustPublicKeyFromBase58(source)),
	)

	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	bin, err := tx.Message.MarshalBinary()
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("%s", err.Error())
	}

	msgBase64 := base64.StdEncoding.EncodeToString(bin)

	fee, err := client.GetFeeForMessage(context.Background(), msgBase64, rpc.CommitmentFinalized)

	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	// int64(*..) is risky as uint64 holds more, however we assume solana fees are <<<
	return SolanaBaseAmount(int64(*fee.Value)), nil
}

// Estimate the fee for a transaction between two cbe controlled wallets
func EstimateFeeUSDCITransaction(source GenericSecureID, dest GenericSecureID, amount USDCBaseAmount) (SolanaBaseAmount, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	dest_wallet, err := GetSolanaWalletByID(dest)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE || dest_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return SolanaBaseAmount(0), fmt.Errorf("will not estimate fee for inactive wallet")
	}

	return estimateFeeUSDCTransaction(source_wallet.PublicKey, dest_wallet.PublicKey, amount)
}

// Estimate the fee for a transaction leaving a cbe controlled wallet
func EstimateFeeUSDCHTransaction(source GenericSecureID, dest string, amount USDCBaseAmount) (SolanaBaseAmount, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return SolanaBaseAmount(0), err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return SolanaBaseAmount(0), fmt.Errorf("will not estimate fee for inactive wallet")
	}

	return estimateFeeUSDCTransaction(source_wallet.PublicKey, dest, amount)
}

// Get real fee paid from a transaction
func GetFeeFromTransaction(txid string) (SolanaBaseAmount, error) {
	client := rpc.New(GetSolRPCAddress())
	tx, err := client.GetTransaction(context.Background(), solana.MustSignatureFromBase58(txid), &rpc.GetTransactionOpts{Encoding: solana.EncodingBase64, Commitment: rpc.CommitmentFinalized})
	if err != nil {
		return SolanaBaseAmount(0), fmt.Errorf("solana - %s", err.Error())
	}

	return SolanaBaseAmount(int64(tx.Meta.Fee)), nil
}

// (Internal) Send a transaction from source to dest signed with private_key for USDC base units amount, returns TXID or err.
// TODO: Have fee paid from central
func newUSDCTransaction(source string, private_key string, destination string, amount USDCBaseAmount) (string, error) {
	client := rpc.New(GetSolRPCAddress())

	priv := solana.MustPrivateKeyFromBase58(private_key)
	src := solana.MustPublicKeyFromBase58(source)
	src_ata, err := GetSolanaWalletATA(source, GetSolUSDCMint())
	if err != nil {
		return "", fmt.Errorf("solana (ata) - %s", err.Error())
	}

	dest_ata, err := GetSolanaWalletATA(destination, GetSolUSDCMint())
	if err != nil {
		return "", fmt.Errorf("solana (ata) - %s", err.Error())
	}

	amx := uint64(amount)

	ix := token.NewTransferInstruction(
		amx,
		solana.MustPublicKeyFromBase58(src_ata),
		solana.MustPublicKeyFromBase58(dest_ata),
		src,
		nil,
	).Build()

	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)

	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{ix},
		recent.Value.Blockhash,
		solana.TransactionPayer(src),
	)

	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(src) {
			return &priv
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	sig, err := client.SendTransaction(
		context.Background(),
		tx,
	)

	if err != nil {
		return "", fmt.Errorf("solana - %s", err.Error())
	}

	return sig.String(), nil
}

// New transaction between two cbe controlled wallets
func NewUSDCITransaction(source GenericSecureID, dest GenericSecureID, amount USDCBaseAmount) (IPaymentID, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return "", err
	}

	if !source_wallet.CanAffordUSDC(amount) {
		return "", fmt.Errorf("source wallet cannot afford transaction")
	}

	dest_wallet, err := GetSolanaWalletByID(dest)
	if err != nil {
		return "", err
	}

	if source_wallet.State != SOLANA_WALLET_STATE_ACTIVE || dest_wallet.State != SOLANA_WALLET_STATE_ACTIVE {
		return "", fmt.Errorf("will not send transaction for inactive wallet")
	}

	txid, err := newUSDCTransaction(source_wallet.PublicKey, source_wallet.PrivateKey, dest_wallet.PublicKey, amount)

	if err != nil {
		return "", err
	}

	ipayid := NewGenericSecureID()
	timestamp := NewTimestamp()
	fee, err := EstimateFeeUSDCITransaction(source, dest, amount) // estimate it first, find it later/don't rely on this field
	if err != nil {
		return "", err
	}

	payment := Payment{
		IPaymentID:          IPaymentID(ipayid),
		SolanaTransactionID: txid,
		SourceUserID:        source_wallet.UserID,
		USDCAmount:          amount,
		SolanaAmount:        0,
		CBEFee:              0,
		SolanaFee:           fee,
		Timestamp:           timestamp,
		Amount:              0,
	}

	id, err := CreatePayment(payment)
	if err != nil {
		return "", err
	}

	return id, nil
}

// New transaction from a cbe controlled wallet to an external wallet
func NewUSDCHTransaction(source GenericSecureID, dest string, amount USDCBaseAmount) (IPaymentID, error) {
	source_wallet, err := GetSolanaWalletByID(source)
	if err != nil {
		return "", err
	}

	if !source_wallet.CanAffordUSDC(amount) {
		return "", fmt.Errorf("source wallet cannot afford transaction")
	}

	dest_wallet, err := GetSolanaWalletATA(dest, GetSolUSDCMint())
	if err != nil {
		return "", err
	}

	txid, err := newUSDCTransaction(source_wallet.PublicKey, source_wallet.PrivateKey, dest_wallet, amount)
	if err != nil {
		return "", err
	}

	ipayid := NewGenericSecureID()
	timestamp := NewTimestamp()
	fee, err := EstimateFeeUSDCHTransaction(source, dest, amount)

	if err != nil {
		return "", err
	}

	payment := Payment{
		IPaymentID:          IPaymentID(ipayid),
		SolanaTransactionID: txid,
		SourceUserID:        source_wallet.UserID,
		USDCAmount:          amount,
		SolanaAmount:        0,
		CBEFee:              0,
		SolanaFee:           fee,
		Timestamp:           timestamp,
	}

	id, err := CreatePayment(payment)
	if err != nil {
		return "", err
	}
	return id, nil
}
