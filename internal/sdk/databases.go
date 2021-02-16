package sdk

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const updateRolePermissionsRetryLimit = 30

func (c *Client) ListDatabases(ctx context.Context) ([]Database, error) {
	var body []Database
	if err := c.request(ctx, http.MethodGet, "/v1/bdbs", nil, &body); err != nil {
		return nil, err
	}

	return body, nil
}

func (c *Client) UpdateDatabase(ctx context.Context, id int, update UpdateDatabase) error {
	if err := c.request(ctx, http.MethodPut, fmt.Sprintf("/v1/bdbs/%d", id), update, nil); err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateDatabaseWithRetry(ctx context.Context, id int, update UpdateDatabase) error {
	for i := 0; i < updateRolePermissionsRetryLimit; i++ {
		err := c.UpdateDatabase(ctx, id, update)
		if err != nil {
			if errors.Is(err, &HttpError{status: http.StatusConflict}) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}

		return nil
	}

	return fmt.Errorf("cannot update database %d roles_permissions - too many retries after conflicts (409)", id)
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
