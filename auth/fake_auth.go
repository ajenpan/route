package auth

type FakeAuth struct {
}

func (a *FakeAuth) TokenAuth(token string) *UserInfo {
	c++
	return &UserInfo{
		Uid:   c,
		Uname: token,
	}
}

func (a *FakeAuth) AccountAuth(account string, password string) *UserInfo {
	c++
	return &UserInfo{
		Uid:   c,
		Uname: account,
	}
}
