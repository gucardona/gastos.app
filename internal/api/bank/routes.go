// Package bank serves bank accounts, statement import, the inbox of imported lines and the categorization rules.
package bank

import (
	"net/http"

	"gastos/internal/api/httpx"
)

// Handler serves the bank routes.
type Handler struct {
	*httpx.Env
}

// Routes registers the bank account, statement import, inbox and categorization rule routes of a wallet.
func (s *Handler) Routes(mux *http.ServeMux, wallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}"
	mux.Handle("GET "+base+"/bank-accounts", wallet(httpx.RoleReader, s.listBankAccounts))
	mux.Handle("POST "+base+"/bank-accounts", wallet(httpx.RoleEditor, s.createBankAccount))
	mux.Handle("PUT "+base+"/bank-accounts/{id}", wallet(httpx.RoleEditor, s.updateBankAccount))
	mux.Handle("DELETE "+base+"/bank-accounts/{id}", wallet(httpx.RoleEditor, s.deleteBankAccount))
	mux.Handle("POST "+base+"/bank-accounts/{id}/import", wallet(httpx.RoleEditor, s.importStatement))

	mux.Handle("GET "+base+"/inbox", wallet(httpx.RoleReader, s.listInbox))
	mux.Handle("POST "+base+"/inbox/accept-suggested", wallet(httpx.RoleEditor, s.acceptSuggested))
	mux.Handle("POST "+base+"/inbox/{id}/accept", wallet(httpx.RoleEditor, s.acceptInboxItem))
	mux.Handle("POST "+base+"/inbox/{id}/ignore", wallet(httpx.RoleEditor, s.ignoreInboxItem))
	mux.Handle("POST "+base+"/inbox/{id}/match", wallet(httpx.RoleEditor, s.matchInboxItem))

	mux.Handle("GET "+base+"/rules", wallet(httpx.RoleReader, s.listRules))
	mux.Handle("POST "+base+"/rules", wallet(httpx.RoleEditor, s.createRule))
	mux.Handle("DELETE "+base+"/rules/{id}", wallet(httpx.RoleEditor, s.deleteRule))
}
