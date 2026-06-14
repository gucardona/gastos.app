package handlers

import (
	"encoding/json"
	"gastos/src/db"
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

func accountResponse(id int64, name, role, ownerName string, splittingEnabled bool, participantIds []int64) models.Account {
	if participantIds == nil {
		participantIds = []int64{}
	}
	return models.Account{
		ID:                  id,
		Name:                name,
		Role:                role,
		OwnerName:           ownerName,
		Permissions:         models.PermissionsForRole(role),
		SplittingEnabled:    splittingEnabled,
		SplitParticipantIds: participantIds,
	}
}

func loadSplitParticipantIds(accountID int64) []int64 {
	rows, err := db.DB.Query(`SELECT user_id FROM account_split_participants WHERE account_id = ? ORDER BY user_id ASC`, accountID)
	if err != nil {
		return []int64{}
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var uid int64
		if rows.Scan(&uid) == nil {
			ids = append(ids, uid)
		}
	}
	return ids
}

func parseSplitParticipantIds(s string) []int64 {
	if s == "" {
		return []int64{}
	}
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}
