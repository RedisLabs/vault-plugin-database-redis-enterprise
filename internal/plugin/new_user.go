package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
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
func (r *RedisEnterpriseDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error) {
	r.logger.Debug("new user", "display", req.UsernameConfig.DisplayName, "role", req.UsernameConfig.RoleName, "statements", req.Statements.Commands)

	if len(req.Statements.Commands) != 1 {
		return dbplugin.NewUserResponse{}, errors.New("one creation statement is required")
	}

	var s statement
	if err := json.Unmarshal([]byte(req.Statements.Commands[0]), &s); err != nil {
		return dbplugin.NewUserResponse{}, errors.New("cannot parse JSON for db role")
	}

	if !s.hasRole() && !s.hasACL() {
		return dbplugin.NewUserResponse{}, fmt.Errorf("no 'role' or 'acl' in creation statement for %s", req.UsernameConfig.RoleName)
	}

	// Generate a username which also includes random data (20 characters) and current epoch (11 characters) and the prefix 'v'
	username, err := r.generateUsername(req.UsernameConfig.DisplayName, req.UsernameConfig.RoleName)
	if err != nil {
		return dbplugin.NewUserResponse{}, fmt.Errorf("cannot generate username: %w", err)
	}

	if !s.hasRole() && s.hasACL() && !r.config.hasFeature("acl_only") {
		return dbplugin.NewUserResponse{}, fmt.Errorf("the ACL only feature has not been enabled for %s. You must specify a role name", req.UsernameConfig.RoleName)
	}

	if !r.config.hasDatabase() && s.hasACL() {
		return dbplugin.NewUserResponse{}, fmt.Errorf("ACL cannot be used when the database has not been specified for %s", req.UsernameConfig.RoleName)
	}

	client := r.simpleClient

	var rid int = -1
	var role_management string
	var aid float64 = -1

	if s.hasRole() {
		role, err := r.client.FindRoleByName(ctx, s.Role)
		if err != nil {
			return dbplugin.NewUserResponse{}, err
		}

		rid = role.UID
		role_management = role.Management
	}

	if s.hasACL() {
		// get the ACL id
		var found bool
		aid, found, err = findACL(*client, s.ACL)
		if err != nil {
			return dbplugin.NewUserResponse{}, fmt.Errorf("cannot get acls: %w", err)
		}
		if !found {
			return dbplugin.NewUserResponse{}, fmt.Errorf("cannot find acl: %s", s.ACL)
		}
		role_management = "db_member"
	}

	if r.config.hasDatabase() {

		// If we have a database we need to:
		// 1. Retrieve the DB and role ids
		// 2. Find the role binding in roles_permissions in the DB definition
		// 3. Create a new role for the user
		// 4. Bind the new role to the same ACL in the database

		db, err := r.client.FindDatabaseByName(ctx, r.config.Database)
		if err != nil {
			return dbplugin.NewUserResponse{}, err
		}

		// Find the referenced role binding in the role
		var bound_aid float64 = -1

		if s.hasRole() {
			b := db.FindPermissionForRole(rid)
			if b != nil {
				bound_aid = float64(b.ACLUID)
			}
		}

		// If the role specified without an ACL and not bound in the database, this is an error
		if s.hasRole() && bound_aid < 0 {
			return dbplugin.NewUserResponse{}, fmt.Errorf("database %s has no binding for role %s", r.config.Database, s.Role)
		}

		// If the role and ACL are specified but unbound in the database, this is an error because it
		// may cause escalation of privileges for other users with the same role already
		if s.hasRole() && s.hasACL() && bound_aid < 0 {
			return dbplugin.NewUserResponse{}, fmt.Errorf("database %s has no binding for role %s", r.config.Database, s.Role)
		}

		// If the role and ACL are specified but the binding in the database is different, this is an error
		if s.hasRole() && s.hasACL() && bound_aid >= 0 && aid != bound_aid {
			return dbplugin.NewUserResponse{}, fmt.Errorf("database %s has a different binding for role %s", r.config.Database, s.Role)
		}

		// If only the ACL is specified, create a new role & role binding
		if !s.hasRole() && s.hasACL() {
			vault_role := r.config.Database + "-" + username
			create_role := map[string]interface{}{
				"name":       vault_role,
				"management": role_management,
			}
			create_role_body, err := json.Marshal(create_role)
			if err != nil {
				return dbplugin.NewUserResponse{}, fmt.Errorf("cannot marshal create role request: %s", err)
			}

			create_role_response_raw, err := client.post("/v1/roles", create_role_body)
			if err != nil {
				return dbplugin.NewUserResponse{}, fmt.Errorf("cannot create role: %s", err)
			}

			var create_role_response interface{}
			err = json.Unmarshal(create_role_response_raw, &create_role_response)
			if err != nil {
				return dbplugin.NewUserResponse{}, err
			}

			// Add the new binding to the same ACL
			new_role_id := create_role_response.(map[string]interface{})["uid"].(float64)
			rid = int(new_role_id)

			var rolesPermissions []interface{}
			for _, perm := range db.RolePermissions {
				rolesPermissions = append(rolesPermissions, map[string]interface{}{
					"role_uid":      perm.RoleUID,
					"redis_acl_uid": perm.ACLUID,
				})
			}

			new_binding := map[string]interface{}{
				"role_uid":      rid,
				"redis_acl_uid": aid,
			}
			rolesPermissions = append(rolesPermissions, new_binding)

			// Update the database
			err = updateRolePermissions(*client, float64(db.UID), rolesPermissions)
			if err != nil {
				return dbplugin.NewUserResponse{}, fmt.Errorf("cannot update role_permissions in database %s: %w", r.config.Database, err)
			}

		}

	}

	// Finally, create the user with the role
	_, err = r.client.CreateUser(ctx, sdk.CreateUser{
		Name:        username,
		Password:    req.Password,
		Roles:       []int{rid},
		EmailAlerts: false,
		AuthMethod:  "regular",
	})
	if err != nil {
		return dbplugin.NewUserResponse{}, err
	}

	// TODO: we need to cleanup created roles if the user can't be created

	return dbplugin.NewUserResponse{Username: username}, nil
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
