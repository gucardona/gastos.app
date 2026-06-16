package models

import "testing"

func TestPermissionsForRole(t *testing.T) {
	tests := []struct {
		role              string
		wantEdit          bool
		wantManageMembers bool
		wantDelete        bool
	}{
		{AccountRoleOwner, true, true, true},
		{AccountRoleEditor, true, false, false},
		{AccountRoleReader, false, false, false},
		{"", false, false, false},
		{"admin", false, false, false},
	}
	for _, tc := range tests {
		p := PermissionsForRole(tc.role)
		if p.CanEdit != tc.wantEdit || p.CanManageMembers != tc.wantManageMembers || p.CanDelete != tc.wantDelete {
			t.Errorf("PermissionsForRole(%q) = %+v, want {CanEdit:%v CanManageMembers:%v CanDelete:%v}",
				tc.role, p, tc.wantEdit, tc.wantManageMembers, tc.wantDelete)
		}
	}
}

func TestIsValidAccountRole(t *testing.T) {
	for _, r := range []string{AccountRoleOwner, AccountRoleEditor, AccountRoleReader} {
		if !IsValidAccountRole(r) {
			t.Errorf("IsValidAccountRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "admin", "superuser", "OWNER"} {
		if IsValidAccountRole(r) {
			t.Errorf("IsValidAccountRole(%q) = true, want false", r)
		}
	}
}

func TestIsValidShareRole(t *testing.T) {
	for _, r := range []string{AccountRoleEditor, AccountRoleReader} {
		if !IsValidShareRole(r) {
			t.Errorf("IsValidShareRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{AccountRoleOwner, "", "admin", "EDITOR"} {
		if IsValidShareRole(r) {
			t.Errorf("IsValidShareRole(%q) = true, want false", r)
		}
	}
}
