package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
)

type LoginClaims struct {
	Channel        string
	UID            string
	AuthCode       string
	StandardClaims jwt.StandardClaims
}

func (l *LoginClaims) Valid() error {
	if l.UID == "" || l.Channel == "" {
		return errors.New("illegal tokens")
	}

	t := time.Now().Unix()
	if t > l.StandardClaims.ExpiresAt {
		return errors.New("token has expired")
	}

	return nil
}
