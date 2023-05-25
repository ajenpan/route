package auth

import (
	"crypto/rsa"
)

type AuthClient struct {
	PK *rsa.PublicKey
}

func (a *AuthClient) TokenAuth(token string) *UserInfo {
	// uid, uname, err := common.VerifyToken(a.PK, token)
	// if err != nil {
	// 	return nil
	// }
	// return &UserInfo{
	// 	Uid:   uid,
	// 	Uname: uname,
	// }
	return nil
}

func (a *AuthClient) AccountAuth(account string, password string) *UserInfo {
	return nil
}
