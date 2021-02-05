package sdk

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) ListDatabases(ctx context.Context) ([]Database, error) {
	var body []Database
	if err := c.request(ctx, http.MethodGet, "/v1/bdbs", nil, &body); err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) GetDatabase(ctx context.Context, id int) (Database, error) {
	var body Database
	if err := c.request(ctx, http.MethodGet, fmt.Sprintf("/v1/bdbs/%d", id), nil, &body); err != nil {
		return Database{}, err
	}

	return body, nil
}

func (c *Client) FindDatabaseByName(ctx context.Context, name string) (Database, error) {
	dbs, err := c.ListDatabases(ctx)
	if err != nil {
		return Database{}, err
	}

	for _, db := range dbs {
		if db.Name == name {
			return db, nil
		}
	}

	return Database{}, fmt.Errorf("unable to find database %s", name)
}
