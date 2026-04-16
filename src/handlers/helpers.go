package handlers

import (
	"encoding/json"
	"gastos/src/models"
	"net/http"
	"strconv"
	"strings"
)

func parseIntPathID(path, prefix string) (int64, error) {
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		return 0, errInvalidID
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidID
	}
	return id, nil
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errInvalidJSON
	}
	return nil
}

func accountResponse(id int64, name, role string) models.Account {
	return models.Account{
		ID:          id,
		Name:        name,
		Role:        role,
		Permissions: models.PermissionsForRole(role),
	}
}
