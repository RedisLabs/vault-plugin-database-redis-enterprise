package sdk

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) ListRoles(ctx context.Context) ([]Role, error) {
	var body []Role
	if err := c.request(ctx, http.MethodGet, "/v1/roles", nil, &body); err != nil {
		return nil, err
	}

	return body, nil
}

func (c Client) CreateRole(ctx context.Context, create CreateRole) (Role, error) {
	var body Role
	if err := c.request(ctx, http.MethodPost, "/v1/roles", create, &body); err != nil {
		return Role{}, err
	}
	return body, nil
}

func (c Client) DeleteRole(ctx context.Context, id int) error {
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/v1/roles/%d", id), nil, nil); err != nil {
		return err
	}
	return nil
}

func (c *Client) FindRoleByName(ctx context.Context, name string) (Role, error) {
	roles, err := c.ListRoles(ctx)
	if err != nil {
		return Role{}, err
	}

	for _, role := range roles {
		if role.Name == name {
			return role, nil
		}
	}

	return Role{}, &RoleNotFoundError{name}
}
