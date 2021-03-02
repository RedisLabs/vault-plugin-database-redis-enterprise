package sdk

import (
	"fmt"
)

type User struct {
	UID               int    `json:"uid"`
	Role              string `json:"role"`
	Roles             []int  `json:"role_uids,omitempty"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	PasswordIssueDate string `json:"password_issue_date"`
}

type CreateUser struct {
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
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

type CreateRole struct {
	Name       string `json:"name"`
	Management string `json:"management"`
}

type Database struct {
	UID             int              `json:"uid"`
	Name            string           `json:"name"`
	RolePermissions []RolePermission `json:"roles_permissions"`
}

type UpdateDatabase struct {
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

type ACL struct {
	UID  int    `json:"uid"`
	Name string `json:"name"`
	ACL  string `json:"acl"`
}

var _ error = &UserNotFoundError{}

type UserNotFoundError struct {
	name string
}

func (u *UserNotFoundError) Error() string {
	return fmt.Sprintf("unable to find user %s", u.name)
}

func (u *UserNotFoundError) Is(target error) bool {
	t, ok := target.(*UserNotFoundError)
	if !ok {
		return false
	}

	return u.name == t.name || t.name == ""
}

var _ error = &RoleNotFoundError{}

type RoleNotFoundError struct {
	name string
}

func (u *RoleNotFoundError) Error() string {
	return fmt.Sprintf("unable to find role %s", u.name)
}

func (u *RoleNotFoundError) Is(target error) bool {
	t, ok := target.(*RoleNotFoundError)
	if !ok {
		return false
	}

	return u.name == t.name || t.name == ""
}

var _ error = &HttpError{}

type HttpError struct {
	method string
	path   string
	status int
	body   string
}

func (h *HttpError) Error() string {
	return fmt.Sprintf("unable to perform request %s %s: %d - %s", h.method, h.path, h.status, h.body)
}

func (h *HttpError) Is(target error) bool {
	t, ok := target.(*HttpError)
	if !ok {
		return false
	}

	return (h.path == t.path || t.path == "") &&
		(h.method == t.method || t.method == "") &&
		(h.body == t.body || t.body == "") &&
		(h.status == t.status || t.status == 0)
}
