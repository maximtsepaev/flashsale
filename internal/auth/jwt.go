package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// В реальном проекте это должно читаться из config/env
var jwtSecret = []byte("super_secret_key")

// Claims — структура, которая будет зашита внутрь JWT-токена
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

const saltLength = 16

// HashPassword принимает чистый пароль, делает соль и возвращает строку формата "соль:хэш"
func HashPassword(password string) (string, error) {
	saltBytes := make([]byte, 16)
	_, err := rand.Read(saltBytes)
	if err != nil {
		return "", err
	}

	saltStr := fmt.Sprintf("%x", saltBytes)

	secretData := []byte(saltStr + password)
	hashBytes := sha256.Sum256(secretData)

	hashStr := fmt.Sprintf("%x", hashBytes)

	resultHash := saltStr + ":" + hashStr

	return resultHash, nil
}

// CheckPasswordHash проверяет пароль, сравнивая его с хэшем из базы
func CheckPasswordHash(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, ":")
	if len(parts) != 2 {
		return false
	}
	saltStr := parts[0]
	hashStr := parts[1]

	secretData := []byte(saltStr + password)
	newHashBytes := sha256.Sum256(secretData)
	newHashStr := fmt.Sprintf("%x", newHashBytes)

	if newHashStr == hashStr {
		return true
	}

	return false
}

// GenerateToken создает JWT-токен для пользователя на 24 часа
func GenerateToken(userID int64) (string, error) {
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateToken проверяет строку токена и возвращает UserID, если всё ок
func ValidateToken(tokenStr string) (int64, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return 0, errors.New("invalid token")
	}

	return claims.UserID, nil
}
