package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
)

// DeleteUser removes a user from the cluster entirely
func (r *redisEnterpriseDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	if err := r.findAndDeleteUser(ctx, req.Username); err != nil {
		return dbplugin.DeleteUserResponse{}, err
	}

	if r.config.supportAclOnly() {
		// There's the _possibility_ that a role was created for this user

		if err := r.findAndDeleteRole(ctx, req.Username); err != nil {
			return dbplugin.DeleteUserResponse{}, err
		}
	}

	return dbplugin.DeleteUserResponse{}, nil
}

func (r *redisEnterpriseDB) findAndDeleteUser(ctx context.Context, username string) error {
	user, err := r.client.FindUserByName(ctx, username)

	if err != nil {
		if errors.Is(err, &sdk.UserNotFoundError{}) {
			// If the user is not found, they may have been deleted manually. We'll assume
			// this is okay and return successfully.
			return nil
		}
		return err
	}

	r.logger.Debug("delete user", "user", username, "uid", user.UID)

	if err := r.client.DeleteUser(ctx, user.UID); err != nil {
		return fmt.Errorf("cannot delete user %s: %w", username, err)
	}

	return nil
}

func (r redisEnterpriseDB) findAndDeleteRole(ctx context.Context, username string) error {
	role, err := r.client.FindRoleByName(ctx, r.generateRoleName(username))
	if err != nil {
		if errors.Is(err, &sdk.RoleNotFoundError{}) {
			// If the role is not found, then either the generated role wasn't created in the first place or its been
			// deleted manually
			return nil
		}
		return err
	}

	r.logger.Debug("delete role", "role", role.Name, "uid", role.UID)

	// Found the role with the expected name, so have to assume it was the generated role
	// Any role permissions associated with the role will be deleted by Redis Enterprise
	if err := r.client.DeleteRole(ctx, role.UID); err != nil {
		return err
	}

	return nil
}
