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

	return Role{}, fmt.Errorf("unable to find role %s", name)
}
