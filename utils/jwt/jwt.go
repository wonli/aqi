package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/spf13/viper"
)

func GenerateToken(uid, authCode, channel string) (string, error) {

	if viper.GetString("jwtSecurity") == "" {
		return "", errors.New("jwt security not configured")
	}

	jwtLifetime := viper.GetDuration("jwtLifetime")
	if jwtLifetime == 0 {
		jwtLifetime = time.Hour * 48
	}

	expire := time.Now().Add(jwtLifetime)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &LoginClaims{
		UID:      uid,
		Channel:  channel,
		AuthCode: authCode,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expire.Unix(),
		},
	})

	return token.SignedString([]byte(viper.GetString("jwtSecurity")))
}

func ValidToken(t string) (*LoginClaims, error) {
	token, err := jwt.ParseWithClaims(t, &LoginClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(viper.GetString("jwtSecurity")), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*LoginClaims); ok && token.Valid {
		return claims, nil
	} else {
		return nil, errors.New("failed to validate token")
	}
}
