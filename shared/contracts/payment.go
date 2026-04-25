package contracts

type FlutterwaveCustomer struct {
	Email string `json:"email"`
}

type FlutterwaveCheckoutRequest struct {
	Amount      int64                `json:"amount"`
	TxRef       string               `json:"tx_ref"`
	Customer    *FlutterwaveCustomer `json:"customer"`
	RedirectUrl string               `json:"redirect_url"`
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
		Status string `json:"status"`
		Amount int64  `json:"amount"`
		TxRef  string `json:"tx_ref"`
	} `json:"data"`
}

type PaystackCheckoutRequest struct {
	Email       string   `json:"email"`
	Amount      int64    `json:"amount"`
	Reference   string   `json:"reference"`
	Channels    []string `json:"channels"`
	CallbackUrl string   `json:"callback_url"`
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
	} `json:"data"`
}
