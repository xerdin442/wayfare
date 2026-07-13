package contracts

type GatewayErrorResponse struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`     // Paystack only
	ErrorID string `json:"error_id,omitempty"` // Flutterwave only
}

type PaymentMetadata struct {
	TripID       string `json:"trip_id"`
	UserID       string `json:"user_id"`
	TripRating   int64  `json:"trip_rating"`
	RiderComment string `json:"rider_comment,omitempty"`
	DriverTip    int64  `json:"driver_tip,omitempty"`
}

type FlutterwaveCustomer struct {
	Email string `json:"email"`
}

type FlutterwaveCheckoutRequest struct {
	Amount   int64                `json:"amount"`
	TxRef    string               `json:"tx_ref"`
	Customer *FlutterwaveCustomer `json:"customer"`
	Meta     *PaymentMetadata     `json:"meta"`
}

type FlutterwaveCheckoutResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Link string `json:"link"`
	} `json:"data"`
}

type PaystackCheckoutRequest struct {
	Email     string   `json:"email"`
	Amount    int64    `json:"amount"`
	Reference string   `json:"reference"`
	Channels  []string `json:"channels"`
	Metadata  string   `json:"metadata"`
}

type PaystackCheckoutResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationUrl string `json:"authorization_url"`
		Reference        string `json:"reference"`
		AccessCode       string `json:"access_code"`
	} `json:"data"`
}

type PaystackBankResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
		Code string `json:"code"`
	} `json:"data"`
}

type AccountDetails struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankName      string `json:"bank_name"`
}

type AccountVerificationResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AccountNumber string `json:"account_number"`
		AccountName   string `json:"account_name"`
	} `json:"data"`
}

type CreateTransferRecipientPayload struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
}

type TransferRecipientResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID            int64  `json:"id"`
		Active        bool   `json:"active"`
		RecipientCode string `json:"recipient_code"`
	} `json:"data"`
}

type TransferDetails struct {
	Amount    int64  `json:"amount"`
	Recipient string `json:"recipient"`
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

type BulkTransferPayload struct {
	Currency  string             `json:"currency"`
	Source    string             `json:"source"`
	Transfers []*TransferDetails `json:"transfers"`
}

type PaystackWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Reference     string `json:"reference"`
		Status        string `json:"status"`
		Amount        int64  `json:"amount"`
		ProcessingFee int64  `json:"fees"`
		Metadata      string `json:"metadata"`
		Recipient     struct {
			Email         string `json:"email"`
			RecipientCode string `json:"recipient_code"`
		} `json:"recipient"`
	} `json:"data"`
}

type FlutterwaveWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Status        string          `json:"status"`
		Amount        float64         `json:"amount"`
		TxRef         string          `json:"tx_ref"`
		ProcessingFee float64         `json:"app_fee"`
		Meta          PaymentMetadata `json:"meta"`
	} `json:"data"`
}
