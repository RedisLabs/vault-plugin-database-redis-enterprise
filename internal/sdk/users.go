package sdk

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) ListUsers(ctx context.Context) ([]User, error) {
	var body []User
	if err := c.request(ctx, http.MethodGet, "/v1/users", nil, &body); err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) CreateUser(ctx context.Context, create CreateUser) (User, error) {
	var body User
	if err := c.request(ctx, http.MethodPost, "/v1/users", create, &body); err != nil {
		return User{}, err
	}

	return body, nil
}

func (c *Client) UpdateUserPassword(ctx context.Context, id int, update UpdateUser) error {
	if err := c.request(ctx, http.MethodPut, fmt.Sprintf("/v1/users/%d", id), update, nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteUser(ctx context.Context, id int) error {
	if err := c.request(ctx, http.MethodDelete, fmt.Sprintf("/v1/users/%d", id), nil, nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) FindUserByName(ctx context.Context, name string) (User, error) {
	users, err := c.ListUsers(ctx)
	if err != nil {
		return User{}, err
	}

	for _, user := range users {
		if user.Name == name {
			return user, nil
		}
	}

	return User{}, &UserNotFoundError{name}
}
