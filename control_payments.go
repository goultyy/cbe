package cbe

// Create new payment
func CreatePayment(payment Payment) (IPaymentID, error) {
	sql, err := ReturnSQLConnection()
	if err != nil {
		return IPaymentID(""), err
	}

	payment.IPaymentID = IPaymentID(NewGenericSecureID())
	payment.Timestamp = NewTimestamp()

	_, err = sql.Exec("INSERT INTO payments (IPaymentID, SolanaTransactionID, SourceUserID, Amount, CBEFee, Timestamp) VALUES (?, ?, ?, ?, ?, ?)", payment.IPaymentID, payment.SolanaTransactionID, payment.SourceUserID, payment.Amount, payment.CBEFee, payment.Timestamp)
	if err != nil {
		return IPaymentID(""), err
	}
	return payment.IPaymentID, nil
}
