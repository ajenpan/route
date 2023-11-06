package auth

import (
	"crypto/rsa"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func VerifyToken(pk *rsa.PublicKey, tokenRaw string) (uint64, string, string, error) {
	claims := make(jwt.MapClaims)
	token, err := jwt.ParseWithClaims(tokenRaw, claims, func(t *jwt.Token) (interface{}, error) {
		return pk, nil
	})
	if err != nil {
		return 0, "", "", err
	}
	if !token.Valid {
		return 0, "", "", fmt.Errorf("invalid token")
	}
	uidstr := claims["uid"]
	uname := claims["aud"]
	role := claims["rid"]
	uid, _ := strconv.ParseUint(uidstr.(string), 10, 64)

	return uid, uname.(string), role.(string), err
}

func GenerateToken(pk *rsa.PrivateKey, uid uint64, uname, role string) (string, error) {
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(24 * time.Hour).Unix()
	claims["iat"] = time.Now().Unix()
	claims["uid"] = strconv.FormatUint(uint64(uid), 10)
	claims["aud"] = uname
	claims["rid"] = role
	claims["iss"] = "hotwave"
	claims["sub"] = "auth"
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(pk)
}
