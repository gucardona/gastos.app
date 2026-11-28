package api

type testCard struct {
	ClosingDay      *int   `json:"closingDay"`
	DueDay          *int   `json:"dueDay"`
	Issuer          string `json:"issuer"`
	CardForm        string `json:"cardForm"`
	Last4           string `json:"last4"`
	BestPurchaseDay int    `json:"bestPurchaseDay"`
}
