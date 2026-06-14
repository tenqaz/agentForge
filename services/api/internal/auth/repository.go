package auth

import (
	"context"
	"database/sql"
	"errors"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Role         Role   `json:"role"`
	PasswordHash string `json:"-"`
}

type Repository struct {
	database *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{database: database}
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := r.database.QueryRowContext(ctx, `
		SELECT id, email, role
		FROM users
		WHERE email = ?;
	`, email).Scan(&user.ID, &user.Email, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (r *Repository) PasswordHashForUser(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.database.QueryRowContext(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = ?;
	`, userID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	return hash, nil
}
