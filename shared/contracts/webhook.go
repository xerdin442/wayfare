package contracts

type PaymentWebhookMetadata struct {
	TxnID string `json:"txn_id"`
}

type FlutterwaveCustomer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type FlutterwaveWebhookData struct {
	TxnRef   string                 `json:"id"`
	Status   string                 `json:"status"`
	Amount   int64                  `json:"amount"`
	Customer FlutterwaveCustomer    `json:"customer"`
	Meta     PaymentWebhookMetadata `json:"meta"`
}

type FlutterwaveWebhookPayload struct {
	Type string                 `json:"type"`
	Data FlutterwaveWebhookData `json:"data"`
}

type PaystackWebhookData struct {
	TxnRef   string                 `json:"reference"`
	Status   string                 `json:"status"`
	Amount   int64                  `json:"amount"`
	Metadata PaymentWebhookMetadata `json:"metadata"`
}

type PaystackWebhookPayload struct {
	Event string              `json:"event"`
	Data  PaystackWebhookData `json:"data"`
}
