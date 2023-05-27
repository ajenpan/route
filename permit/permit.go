package permit

//TODO list:
//1. read from config
//2. read from other service

type Permit interface {
	RoleGroups(role string) []string
	CallEnable(role string, msgid uint32) bool
	ForwardEnable(sourceRole, targetRole string, msgid uint32) bool
}

type LocalPermit struct{}

func (LocalPermit) RoleGroups(role string) []string {
	switch role {
	case "admin":
		return []string{"admin"}
	case "user":
		return []string{"user"}
	}
	return []string{}
}

func (LocalPermit) CallEnable(role string, msgid uint32) bool {
	return true
}

func (LocalPermit) ForwardEnable(sourceRole, targetRole string, msgid uint32) bool {
	return true
}
