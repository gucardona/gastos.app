package models

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
}

type Expense struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Date        string  `json:"date"`
}

type Income struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Date        string  `json:"date"`
}

type Goal struct {
	ID       int64   `json:"id"`
	UserID   int64   `json:"-"`
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}
