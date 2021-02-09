package plugin

import (
	"context"
	"fmt"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

// UpdateUser changes a user's password
func (r *RedisEnterpriseDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error) {
	if req.Password == nil {
		return dbplugin.UpdateUserResponse{}, nil
	}

	user, err := r.client.FindUserByName(ctx, req.Username)

	if err != nil {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("cannot find user %s: %w", req.Username, err)
	}

	r.logger.Debug("change password", "user", req.Username, "uid", user.UID)

	if err := r.client.UpdateUserPassword(ctx, user.UID, sdk.UpdateUser{Password: req.Password.NewPassword}); err != nil {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("cannot change user password: %w", err)
	}
	return dbplugin.UpdateUserResponse{}, nil
}
