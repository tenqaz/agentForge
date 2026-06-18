package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const SessionCookieName = "agentforge_session"

var ErrInvalidSession = errors.New("invalid session")

type SessionClaims struct {
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SessionManager struct {
	secret   []byte
	duration time.Duration
	secure   bool
}

func NewSessionManager(secret string, secure bool) *SessionManager {
	return &SessionManager{
		secret:   []byte(secret),
		duration: 7 * 24 * time.Hour,
		secure:   secure,
	}
}

func (m *SessionManager) SetSessionCookie(w http.ResponseWriter, user User) error {
	claims := SessionClaims{
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(m.duration),
	}
	token, err := m.sign(claims)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  claims.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secure,
	})
	return nil
}

func (m *SessionManager) ParseRequest(r *http.Request) (SessionClaims, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return SessionClaims{}, ErrInvalidSession
	}
	return m.parse(cookie.Value)
}

func (m *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0).UTC(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   m.secure,
	})
}

func (m *SessionManager) sign(claims SessionClaims) (string, error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := m.signature(encodedPayload)
	return encodedPayload + "." + signature, nil
}

func (m *SessionManager) parse(token string) (SessionClaims, error) {
	encodedPayload, encodedSignature, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" || encodedSignature == "" {
		return SessionClaims{}, ErrInvalidSession
	}
	expectedSignature := m.signature(encodedPayload)
	if !hmac.Equal([]byte(encodedSignature), []byte(expectedSignature)) {
		return SessionClaims{}, ErrInvalidSession
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return SessionClaims{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	var claims SessionClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return SessionClaims{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	if claims.UserID == "" {
		return SessionClaims{}, ErrInvalidSession
	}
	if !claims.ExpiresAt.After(time.Now().UTC()) {
		return SessionClaims{}, ErrInvalidSession
	}
	return claims, nil
}

func (m *SessionManager) signature(encodedPayload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
