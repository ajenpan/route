package auth

type UserInfo struct {
	UId   uint64
	UName string
	URole string
}

func (u *UserInfo) UserID() uint64 {
	return u.UId
}
func (u *UserInfo) UserRole() string {
	return u.URole
}
func (u *UserInfo) UserName() string {
	return u.UName
}
