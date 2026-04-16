package handlers

import (
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
)

func accountCanEdit(r *http.Request) bool {
	perms := models.PermissionsForRole(middleware.AccountRoleFromContext(r.Context()))
	return perms.CanEdit
}

func requireAccountEdit(w http.ResponseWriter, r *http.Request) bool {
	if accountCanEdit(r) {
		return true
	}
	jsonError(w, "Acesso negado para editar esta conta", http.StatusForbidden)
	return false
}
