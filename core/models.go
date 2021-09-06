package core

type ListType byte

const (
	_                  = iota
	Following ListType = iota
	Followers ListType = iota
)

type UserContainer struct {
	Users []*User `json:"users"`
}

type User struct {
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

type UserMap struct {
	userID   string
	m        map[string]*User
	ListType ListType
}
