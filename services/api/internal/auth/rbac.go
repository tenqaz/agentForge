package auth

import "errors"

var ErrForbidden = errors.New("forbidden")

func RequireAdmin(u User) error {
	if u.Role != RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func RequireAgentOwner(u User, ownerUserID string) error {
	if u.Role == RoleAdmin || u.ID == ownerUserID {
		return nil
	}
	return ErrForbidden
}

func CanViewTemplate(u User, status string) bool {
	return u.Role == RoleAdmin || status == "published"
}
