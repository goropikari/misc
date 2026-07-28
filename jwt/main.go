package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var secret = []byte(getenv("JWT_SECRET", "dev-only-secret-change-me"))

var b64 = base64.RawURLEncoding

type Claims struct {
	Subject string `json:"sub"`
	Issuer  string `json:"iss"`
	Expires int64  `json:"exp"`
	Issued  int64  `json:"iat"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func main() {
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/me", auth(meHandler))

	log.Println("JWT sample listening on http://localhost:8080")
	log.Println("demo credentials: alice / password")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username != "alice" || req.Password != "password" {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	now := time.Now()
	token, err := sign(Claims{
		Subject: req.Username,
		Issuer:  "jwt-sample",
		Issued:  now.Unix(),
		Expires: now.Add(15 * time.Minute).Unix(),
	})
	if err != nil {
		http.Error(w, "could not create token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"access_token": token, "token_type": "Bearer"})
}

func meHandler(w http.ResponseWriter, r *http.Request, claims Claims) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "authenticated",
		"user":    claims.Subject,
	})
}

func auth(next func(http.ResponseWriter, *http.Request, Claims)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Bearer token required", http.StatusUnauthorized)
			return
		}

		claims, err := verify(parts[1])
		if err != nil {
			http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
			return
		}
		next(w, r, claims)
	}
}

func sign(claims Claims) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := b64.EncodeToString(header) + "." + b64.EncodeToString(payload)
	return unsigned + "." + b64.EncodeToString(mac(unsigned)), nil
}

func verify(token string) (Claims, error) {
	var claims Claims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, errors.New("malformed token")
	}

	var header struct {
		Alg string `json:"alg"`
	}
	headerBytes, err := b64.DecodeString(parts[0])
	if err != nil || json.Unmarshal(headerBytes, &header) != nil || header.Alg != "HS256" {
		return claims, errors.New("unsupported algorithm")
	}

	expected := mac(parts[0] + "." + parts[1])
	actual, err := b64.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(expected, actual) != 1 {
		return claims, errors.New("invalid signature")
	}

	payloadBytes, err := b64.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payloadBytes, &claims) != nil {
		return claims, errors.New("invalid payload")
	}
	if claims.Subject == "" || claims.Issuer != "jwt-sample" || time.Now().Unix() >= claims.Expires {
		return claims, errors.New("expired or invalid claims")
	}
	return claims, nil
}

func mac(value string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(value))
	return h.Sum(nil)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
