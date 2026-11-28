// Package ledger serves the money: transactions and their splits, categories, payment methods and cards,
// budgets, recurrences, settlements and the monthly summary.
package ledger

import (
	"net/http"

	"gastos/internal/api/httpx"
)

// Handler serves the ledger routes.
type Handler struct {
	*httpx.Env
}

// Routes registers the category, payment method, transaction, recurrence, settlement, budget and summary routes
// of a wallet on mux. inWallet wraps a route that needs membership of the wallet in the path; no ledger route
// is session-only, so the user wrapper is not used.
func (s *Handler) Routes(mux *http.ServeMux, _ func(httpx.HandlerFunc) http.Handler, inWallet httpx.RouteWrap) {
	s.catalogRoutes(mux, inWallet)
	s.transactionRoutes(mux, inWallet)
	s.recurrenceRoutes(mux, inWallet)
	s.settlementRoutes(mux, inWallet)
	s.budgetRoutes(mux, inWallet)
	s.summaryRoutes(mux, inWallet)
}

// catalogRoutes registers the category and payment method routes of a wallet.
func (s *Handler) catalogRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}"
	mux.Handle("GET "+base+"/categories", inWallet(httpx.RoleReader, s.listCategories))
	mux.Handle("POST "+base+"/categories", inWallet(httpx.RoleEditor, s.createCategory))
	mux.Handle("PUT "+base+"/categories/{id}", inWallet(httpx.RoleEditor, s.updateCategory))
	mux.Handle("DELETE "+base+"/categories/{id}", inWallet(httpx.RoleEditor, s.deleteCategory))
	mux.Handle("GET "+base+"/payment-methods", inWallet(httpx.RoleReader, s.listPaymentMethods))
	mux.Handle("POST "+base+"/payment-methods", inWallet(httpx.RoleEditor, s.createPaymentMethod))
	mux.Handle("PUT "+base+"/payment-methods/{id}", inWallet(httpx.RoleEditor, s.updatePaymentMethod))
	mux.Handle("DELETE "+base+"/payment-methods/{id}", inWallet(httpx.RoleEditor, s.deletePaymentMethod))
}

// transactionRoutes registers the transaction routes of a wallet.
func (s *Handler) transactionRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/transactions"
	mux.Handle("GET "+base, inWallet(httpx.RoleReader, s.listTransactions))
	mux.Handle("POST "+base, inWallet(httpx.RoleEditor, s.createTransaction))
	mux.Handle("PUT "+base+"/{id}", inWallet(httpx.RoleEditor, s.updateTransaction))
	mux.Handle("DELETE "+base+"/{id}", inWallet(httpx.RoleEditor, s.deleteTransaction))
}

// recurrenceRoutes registers the recurrence routes of a wallet.
func (s *Handler) recurrenceRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/recurrences"
	mux.Handle("GET "+base, inWallet(httpx.RoleReader, s.listRecurrences))
	mux.Handle("POST "+base, inWallet(httpx.RoleEditor, s.createRecurrence))
	mux.Handle("PUT "+base+"/{id}", inWallet(httpx.RoleEditor, s.updateRecurrence))
	mux.Handle("DELETE "+base+"/{id}", inWallet(httpx.RoleEditor, s.deleteRecurrence))
}

// settlementRoutes registers the settlement routes of a wallet.
func (s *Handler) settlementRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/settlements"
	mux.Handle("GET "+base, inWallet(httpx.RoleReader, s.listSettlements))
	mux.Handle("POST "+base, inWallet(httpx.RoleEditor, s.createSettlement))
	mux.Handle("PUT "+base+"/{id}", inWallet(httpx.RoleEditor, s.updateSettlement))
	mux.Handle("DELETE "+base+"/{id}", inWallet(httpx.RoleEditor, s.deleteSettlement))
}

// budgetRoutes registers the budget routes of a wallet.
func (s *Handler) budgetRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	const base = "/api/wallets/{walletID}/budgets"
	mux.Handle("GET "+base, inWallet(httpx.RoleReader, s.listBudgets))
	mux.Handle("PUT "+base+"/{categoryID}", inWallet(httpx.RoleEditor, s.putBudget))
	mux.Handle("DELETE "+base+"/{categoryID}", inWallet(httpx.RoleEditor, s.deleteBudget))
}

// summaryRoutes registers the monthly summary route of a wallet.
func (s *Handler) summaryRoutes(mux *http.ServeMux, inWallet httpx.RouteWrap) {
	mux.Handle("GET /api/wallets/{walletID}/summary", inWallet(httpx.RoleReader, s.summary))
}
