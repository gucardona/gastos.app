package models

const (
	AccountRoleOwner  = "owner"
	AccountRoleEditor = "editor"
	AccountRoleReader = "reader"
)

type User struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"-"`
}

type Expense struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"-"`
	PaidByUserID       int64   `json:"paidByUserId"`
	AccountID          int64   `json:"-"`
	Amount             float64 `json:"amount"`
	Description        string  `json:"description"`
	Category           string  `json:"category"`
	Payment            string  `json:"payment"`
	Date               string  `json:"date"`
	RecurringExpenseID *int64  `json:"recurringExpenseId,omitempty"`
	Splits             []Split `json:"splits"`
}

type Income struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	AccountID   int64   `json:"-"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Date        string  `json:"date"`
}

type Goal struct {
	ID        int64   `json:"id"`
	UserID    int64   `json:"-"`
	AccountID int64   `json:"-"`
	Category  string  `json:"category"`
	Limit     float64 `json:"limit"`
}

type Category struct {
	ID           int64  `json:"id"`
	AccountID    int64  `json:"-"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Color        string `json:"color"`
	Essentiality string `json:"essentiality"`
	SortOrder    int    `json:"sortOrder"`
}

type PaymentMethod struct {
	ID        int64  `json:"id"`
	AccountID int64  `json:"-"`
	Key       string `json:"key"`
	Icon      string `json:"icon"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type RecurringExpense struct {
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	AccountID   int64   `json:"-"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Frequency   string  `json:"frequency"`
	DayOfMonth  int     `json:"dayOfMonth"`
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
	Enabled     bool    `json:"enabled"`
	Splits      []Split `json:"splits"`
}

type Split struct {
	UserID     int64   `json:"userId"`
	Name       string  `json:"name,omitempty"`
	Email      string  `json:"email,omitempty"`
	Percentage float64 `json:"percentage"`
	Amount     float64 `json:"amount,omitempty"`
}

type SplitPayment struct {
	ID             int64   `json:"id"`
	AccountID      int64   `json:"-"`
	PayerUserID    int64   `json:"payerUserId"`
	PayerName      string  `json:"payerName,omitempty"`
	ReceiverUserID int64   `json:"receiverUserId"`
	ReceiverName   string  `json:"receiverName,omitempty"`
	Amount         float64 `json:"amount"`
	Date           string  `json:"date"`
	Note           string  `json:"note"`
}

type AccountPermissions struct {
	CanEdit          bool `json:"canEdit"`
	CanManageMembers bool `json:"canManageMembers"`
	CanDelete        bool `json:"canDelete"`
}

type Account struct {
	ID                  int64              `json:"id"`
	Name                string             `json:"name"`
	Role                string             `json:"role"`
	OwnerName           string             `json:"ownerName"`
	Permissions         AccountPermissions `json:"permissions"`
	SplittingEnabled    bool               `json:"splittingEnabled"`
	SplitParticipantIds []int64            `json:"splitParticipantIds"`
}

type AccountMember struct {
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

func PermissionsForRole(role string) AccountPermissions {
	switch role {
	case AccountRoleOwner:
		return AccountPermissions{
			CanEdit:          true,
			CanManageMembers: true,
			CanDelete:        true,
		}
	case AccountRoleEditor:
		return AccountPermissions{
			CanEdit:          true,
			CanManageMembers: false,
			CanDelete:        false,
		}
	default:
		return AccountPermissions{}
	}
}

func IsValidAccountRole(role string) bool {
	switch role {
	case AccountRoleOwner, AccountRoleEditor, AccountRoleReader:
		return true
	default:
		return false
	}
}

func IsValidShareRole(role string) bool {
	switch role {
	case AccountRoleEditor, AccountRoleReader:
		return true
	default:
		return false
	}
}
