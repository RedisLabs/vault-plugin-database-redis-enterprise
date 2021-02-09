package sdk

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) ListACLs(ctx context.Context) ([]ACL, error) {
	var body []ACL
	if err := c.request(ctx, http.MethodGet, "/v1/redis_acls", nil, &body); err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) FindACLByName(ctx context.Context, name string) (*ACL, error) {
	acls, err := c.ListACLs(ctx)
	if err != nil {
		return nil, err
	}

	for _, acl := range acls {
		if acl.Name == name {
			return &acl, nil
		}
	}

	return nil, fmt.Errorf("unable to find acl %s", name)
}
