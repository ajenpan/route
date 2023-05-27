package proto

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.String()
}
