package auth

type UserInfo struct {
	Uid   int64
	Uname string
}

type Auth interface {
	TokenAuth(token string) *UserInfo
	AccountAuth(account string, password string) *UserInfo
}
