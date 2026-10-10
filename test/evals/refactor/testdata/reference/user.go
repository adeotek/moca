// Package shop holds the account and order models.
package shop

import (
	"errors"
	"strings"
)

type User struct {
	Name  string
	Email string
}

// NewUser validates and normalizes a user.
func NewUser(name, email string) (User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return User{}, errors.New("name is required")
	}
	e, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	return User{Name: name, Email: e}, nil
}
