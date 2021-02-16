package sdk

import (
	"context"
	"net/http"
)

func (c *Client) GetCluster(ctx context.Context) (Cluster, error) {
	var body Cluster
	if err := c.request(ctx, http.MethodGet, "/v1/cluster", nil, &body); err != nil {
		return Cluster{}, err
	}

	return body, nil
}
