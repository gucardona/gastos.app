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
	ID          int64   `json:"id"`
	UserID      int64   `json:"-"`
	AccountID   int64   `json:"-"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Date        string  `json:"date"`
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

type AccountPermissions struct {
	CanEdit          bool `json:"canEdit"`
	CanManageMembers bool `json:"canManageMembers"`
	CanDelete        bool `json:"canDelete"`
}

type Account struct {
	ID          int64              `json:"id"`
	Name        string             `json:"name"`
	Role        string             `json:"role"`
	Permissions AccountPermissions `json:"permissions"`
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
