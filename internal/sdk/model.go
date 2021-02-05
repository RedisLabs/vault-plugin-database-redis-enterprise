package sdk

import "fmt"

type User struct {
	UID               int    `json:"uid"`
	Role              string `json:"role"`
	Name              string `json:"name"`
	PasswordIssueDate string `json:"password_issue_date"`
}

type CreateUser struct {
	Name        string `json:"name,omitempty"`
	Password    string `json:"password,omitempty"`
	Roles       []int  `json:"role_uids,omitempty"`
	EmailAlerts bool   `json:"email_alerts"`
	AuthMethod  string `json:"auth_method,omitempty"`
}

type UpdateUser struct {
	Password string `json:"password,omitempty"`
}

type Role struct {
	UID        int    `json:"uid"`
	Name       string `json:"name"`
	Management string `json:"management"`
}

type Database struct {
	UID             int              `json:"uid"`
	Name            string           `json:"name"`
	RolePermissions []RolePermission `json:"roles_permissions"`
}

func (d Database) FindPermissionForRole(uid int) *RolePermission {
	for _, permission := range d.RolePermissions {
		if permission.RoleUID == uid {
			return &permission
		}
	}
	return nil
}

type RolePermission struct {
	RoleUID int `json:"role_uid"`
	ACLUID  int `json:"redis_acl_uid"`
}

type Cluster struct {
	Name string `json:"name"`
}

var _ error = &UserNotFoundError{}

type UserNotFoundError struct {
	name string
}

func (u *UserNotFoundError) Error() string {
	return fmt.Sprintf("unable to find user %s", u.name)
}
