package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

var ErrUserNotFound = errors.New("user not found")
var ErrInvalidEmail = errors.New("invalid email")
var ErrInvalidPassword = errors.New("invalid password")
var ErrEmailAlreadyExists = errors.New("email already exists")

type CreateUserParams struct {
	Email    string
	Password string
	Role     Role
}

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

func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	email, err := normalizeEmail(params.Email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(params.Password); err != nil {
		return User{}, err
	}

	role := params.Role
	if role == "" {
		role = RoleUser
	}

	hash, err := HashPassword(params.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	user := User{
		ID:    uuid.NewString(),
		Email: email,
		Role:  role,
	}

	_, err = r.database.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES (?, ?, ?, ?);
	`, user.ID, user.Email, hash, user.Role)
	if isUniqueConstraint(err) {
		return User{}, ErrEmailAlreadyExists
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
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

func (r *Repository) FindUserByID(ctx context.Context, userID string) (User, error) {
	var user User
	err := r.database.QueryRowContext(ctx, `
		SELECT id, email, role
		FROM users
		WHERE id = ?;
	`, userID).Scan(&user.ID, &user.Email, &user.Role)
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

func (r *Repository) EnsureDefaultAdmin(ctx context.Context) error {
	_, err := r.FindUserByEmail(ctx, "admin@123.com")
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return err
	}

	hash, err := HashPassword("admin")
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = r.database.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES (?, ?, ?, ?);
	`, "admin", "admin@123.com", hash, RoleAdmin)

	if isUniqueConstraint(err) {
		return nil
	}
	return err
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil || parsed.Address != normalized {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return ErrInvalidPassword
	}

	var hasLetter bool
	var hasDigit bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return ErrInvalidPassword
	}
	return nil
}
