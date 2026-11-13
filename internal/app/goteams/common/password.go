// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes the password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword checks whether the given password matches the stored hash.
func VerifyPassword(storedHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil
}
