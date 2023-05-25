package auth

import (
	"crypto/rsa"
)

type LocalAuth struct {
	PK *rsa.PublicKey
}

func (a *LocalAuth) TokenAuth(token string) *UserInfo {
	if len(token) < 10 {
		return nil
	}
	return nil
	// uid, uname, err := common.VerifyToken(a.PK, token)
	// if err != nil {
	// 	return nil
	// }

	// return &UserInfo{
	// 	Uid:   uid,
	// 	Uname: uname,
	// }
}

var c = int64(0)

func (a *LocalAuth) AccountAuth(account string, password string) *UserInfo {
	c++
	return &UserInfo{
		Uid:   c,
		Uname: account,
	}
}
