package auth

type UserInfo struct {
	UId   uint32
	UName string
	URole string
}

func (u *UserInfo) UserID() uint32 {
	if u == nil {
		return 0
	}
	return u.UId
}
func (u *UserInfo) UserRole() string {
	return u.URole
}
func (u *UserInfo) UserName() string {
	return u.UName
}
