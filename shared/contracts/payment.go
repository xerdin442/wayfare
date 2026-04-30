package contracts

type GatewayErrorResponse struct {
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
	Type    string `json:"type,omitempty"`     // Paystack only
	ErrorID string `json:"error_id,omitempty"` // Flutterwave only
}

type PaymentMetadata struct {
	TripID       string `json:"trip_id"`
	TripRating   int64  `json:"trip_rating"`
	RiderComment string `json:"rider_comment,omitempty"`
}

type FlutterwaveCustomer struct {
	Email string `json:"email"`
}

type FlutterwaveCheckoutRequest struct {
	Amount      int64                `json:"amount"`
	TxRef       string               `json:"tx_ref"`
	Customer    *FlutterwaveCustomer `json:"customer"`
	RedirectUrl string               `json:"redirect_url"`
	Meta        *PaymentMetadata     `json:"meta"`
}

type FlutterwaveCheckoutResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Link string `json:"link"`
	} `json:"data"`
}

type FlutterwaveWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Status string           `json:"status"`
		Amount int64            `json:"amount"`
		TxRef  string           `json:"tx_ref"`
		Meta   *PaymentMetadata `json:"meta"`
	} `json:"data"`
}

type PaystackCheckoutRequest struct {
	Email       string   `json:"email"`
	Amount      int64    `json:"amount"`
	Reference   string   `json:"reference"`
	Channels    []string `json:"channels"`
	CallbackUrl string   `json:"callback_url"`
	Metadata    string   `json:"metadata"`
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

type PaystackWebhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Reference string `json:"reference"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
		Metadata  string `json:"metadata"`
	} `json:"data"`
}
