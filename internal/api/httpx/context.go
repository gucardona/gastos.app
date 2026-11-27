package httpx

import "context"

// ctxKey keys the values the middleware stores in a request context.
type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxWalletID
	ctxRole
)

// Role is a member's permission level in a wallet; a higher role can do everything a lower one can.
type Role int

const (
	// RoleReader can read a wallet.
	RoleReader Role = iota + 1
	// RoleEditor can also change its data.
	RoleEditor
	// RoleOwner can also manage the wallet and its members.
	RoleOwner
)

// String returns the role as stored and sent to the client.
func (r Role) String() string {
	switch r {
	case RoleOwner:
		return "owner"
	case RoleEditor:
		return "editor"
	case RoleReader:
		return "reader"
	}
	return ""
}

// ParseRole converts a stored role name to a role.
func ParseRole(s string) (Role, bool) {
	switch s {
	case "owner":
		return RoleOwner, true
	case "editor":
		return RoleEditor, true
	case "reader":
		return RoleReader, true
	}
	return 0, false
}

// UserIDFrom returns the signed-in user's id, set by the user middleware.
func UserIDFrom(ctx context.Context) int64 { v, _ := ctx.Value(ctxUserID).(int64); return v }

// WalletIDFrom returns the wallet id of the route, set by the wallet middleware.
func WalletIDFrom(ctx context.Context) int64 { v, _ := ctx.Value(ctxWalletID).(int64); return v }

// RoleFrom returns the user's role in the route's wallet, set by the wallet middleware.
func RoleFrom(ctx context.Context) Role { v, _ := ctx.Value(ctxRole).(Role); return v }

// WithUser returns ctx carrying the signed-in user's id.
func WithUser(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxUserID, id)
}

// WithWallet returns ctx carrying the route's wallet id and the user's role in it.
func WithWallet(ctx context.Context, id int64, role Role) context.Context {
	ctx = context.WithValue(ctx, ctxWalletID, id)
	return context.WithValue(ctx, ctxRole, role)
}
