package auth

type UserInfo struct {
	ID   uint32
	Name string
	Role string
}

type Auth interface {
	TokenAuth(token string) (*UserInfo, error)
}
