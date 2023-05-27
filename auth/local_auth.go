package auth

import (
	"crypto/rsa"
)

type LocalAuth struct {
	PK *rsa.PublicKey
}

func (a *LocalAuth) TokenAuth(token string) (*UserInfo, error) {
	uid, uname, role, err := VerifyToken(a.PK, token)
	if err != nil {
		return nil, err
	}
	return &UserInfo{
		ID:   uid,
		Name: uname,
		Role: role,
	}, nil
}
