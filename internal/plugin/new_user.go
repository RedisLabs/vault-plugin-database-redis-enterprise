package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
)

// NewUser creates a new user and authentication credentials in the cluster.
// The statement is required to be JSON with the structure:
// {
//    "role" : "role_name"
// }
// The role name is must exist the cluster before the user can be created.
// If a database configuration exists, the role must be bound to an ACL in the database.
//
// or
// {
//    "acl" : "acl_name"
// }
// The acl name is must exist the cluster before the user can be created.
// The acl option can only be used with a database.
func (r *redisEnterpriseDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (_ dbplugin.NewUserResponse, err error) {
	r.logger.Debug("new user", "display", req.UsernameConfig.DisplayName, "role", req.UsernameConfig.RoleName, "statements", req.Statements.Commands)

	if len(req.Statements.Commands) != 1 {
		return dbplugin.NewUserResponse{}, errors.New("one creation statement is required")
	}

	var s statement
	if err := json.Unmarshal([]byte(req.Statements.Commands[0]), &s); err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("cannot parse JSON for db role: %w", err)
	}

	if !s.hasRole() && !s.hasACL() {
		return dbplugin.NewUserResponse{}, fmt.Errorf("no 'role' or 'acl' in creation statement for %s", req.UsernameConfig.RoleName)
	}

	// Generate a username which also includes random data (20 characters) and current epoch (11 characters) and the prefix 'v'.
	// Note that the username is used when generating a role, so the maximum length of the username must allow
	// space for a database name (up to 63 characters) and a hyphen (maximum username length supported by Redis
	// is 256)
	username, err := credsutil.GenerateUsername(
		credsutil.DisplayName(req.UsernameConfig.DisplayName, 50),
		credsutil.RoleName(req.UsernameConfig.RoleName, 50),
		credsutil.MaxLength(192),
		credsutil.ToLower(),
	)
	if err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("cannot generate username: %w", err)
	}

	if !s.hasRole() && s.hasACL() && !r.config.supportAclOnly() {
		return dbplugin.NewUserResponse{}, fmt.Errorf("the ACL only feature has not been enabled for %s. You must specify a role name", req.UsernameConfig.RoleName)
	}

	if !r.config.hasDatabase() && s.hasACL() {
		return dbplugin.NewUserResponse{}, fmt.Errorf("ACL cannot be used when the database has not been specified for %s", req.UsernameConfig.RoleName)
	}

	var role sdk.Role

	if s.hasRole() {
		var err error
		role, err = r.client.FindRoleByName(ctx, s.Role)
		if err != nil {
			return dbplugin.NewUserResponse{}, err
		}

		if r.config.hasDatabase() {
			db, err := r.client.FindDatabaseByName(ctx, r.config.Database)
			if err != nil {
				return dbplugin.NewUserResponse{}, err
			}

			perm := db.FindPermissionForRole(role.UID)

			// If the role specified without an ACL and not bound in the database, this is an error
			// or
			// If the role and ACL are specified but unbound in the database, this is an error because it
			// may cause escalation of privileges for other users with the same role already
			if perm == nil {
				return dbplugin.NewUserResponse{}, fmt.Errorf("database %s has no binding for role %s", r.config.Database, s.Role)
			}

			if s.hasACL() {
				acl, err := r.client.FindACLByName(ctx, s.ACL)
				if err != nil {
					return dbplugin.NewUserResponse{}, err
				}

				// If the role and ACL are specified but the binding in the database is different, this is an error
				if acl.UID != perm.ACLUID {
					return dbplugin.NewUserResponse{}, fmt.Errorf("database %s has a different binding for role %s", r.config.Database, s.Role)
				}
			}
		}
	} else if s.hasACL() {
		role, err = r.generateRole(ctx, s.ACL, r.generateRoleName(username), "db_member")
		if err != nil {
			return dbplugin.NewUserResponse{}, err
		}

		defer r.cleanUpGeneratedRoleOnError(&err, role)
	}

	// Finally, create the user with the role
	_, err = r.client.CreateUser(ctx, sdk.CreateUser{
		Name:        username,
		Password:    req.Password,
		Roles:       []int{role.UID},
		EmailAlerts: false,
		AuthMethod:  "regular",
	})
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	return dbplugin.NewUserResponse{Username: username}, nil
}

func (r redisEnterpriseDB) generateRoleName(username string) string {
	return r.config.Database + "-" + username
}

func (r *redisEnterpriseDB) generateRole(ctx context.Context, aclName string, roleName string, roleManagement string) (_ sdk.Role, err error) {
	r.databaseRolePermissions.Lock()
	defer r.databaseRolePermissions.Unlock()

	acl, err := r.client.FindACLByName(ctx, aclName)
	if err != nil {
		return sdk.Role{}, err
	}

	role, err := r.client.CreateRole(ctx, sdk.CreateRole{
		Name:       roleName,
		Management: roleManagement,
	})
	if err != nil {
		return sdk.Role{}, err
	}

	defer r.cleanUpGeneratedRoleOnError(&err, role)

	db, err := r.client.FindDatabaseByName(ctx, r.config.Database)
	if err != nil {
		return sdk.Role{}, err
	}

	permissions := append(db.RolePermissions, sdk.RolePermission{
		RoleUID: role.UID,
		ACLUID:  acl.UID,
	})
	if err := r.client.UpdateDatabaseWithRetry(ctx, db.UID, sdk.UpdateDatabase{
		RolePermissions: permissions,
	}); err != nil {
		return sdk.Role{}, err
	}

	return role, nil
}

func (r *redisEnterpriseDB) cleanUpGeneratedRoleOnError(originalErr *error, role sdk.Role) {
	if *originalErr == nil {
		return
	}

	// Can't use the 'real' context as there's the possibility that the problem is the context timed out
	// so wouldn't be able to roll back
	// Any role permissions associated with the role will be deleted by Redis Enterprise
	if err := r.client.DeleteRole(context.TODO(), role.UID); err != nil {
		*originalErr = multierror.Append(*originalErr, err)
	}
}

type statement struct {
	Role string `json:"role"`
	ACL  string `json:"acl"`
}

func (s statement) hasRole() bool {
	return s.Role != ""
}

func (s statement) hasACL() bool {
	return s.ACL != ""
}
