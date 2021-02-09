package plugin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/version"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	"github.com/mitchellh/mapstructure"
)

const redisEnterpriseTypeName = "redisenterprise"

// This REST client just handles the raw requests with JSON and nothing more.
type SimpleRESTClient struct {
	BaseURL      string
	Username     string
	Password     string
	RoundTripper http.RoundTripper
}

// The timeout for the REST client requests.
const timeout = 60

// getURL computes the URL path relative to the base URL and returns it as a string
func (c *SimpleRESTClient) getURL(apiPath string) string {
	return fmt.Sprintf("%s/%s", c.BaseURL, apiPath)
}

func (c *SimpleRESTClient) Initialise(url string, username string, password string) {
	c.BaseURL = strings.TrimSuffix(url, "/")
	c.Username = username
	c.Password = password
}

// request performs an HTTP(S) request, adding various options like authentication. The
// response is return as a tuple that includes the body of the response message and
// status code.
func (c *SimpleRESTClient) request(req *http.Request) (responseBytes []byte, statusCode int, err error) {
	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	httpClient := http.Client{Timeout: timeout * time.Second, Transport: c.RoundTripper}

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, -1, err
	}

	responseBytes, err = ioutil.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return nil, -1, err
	}
	return responseBytes, response.StatusCode, nil
}

// get performs an HTTP get and returns a JSON response message
func (c *SimpleRESTClient) get(apiPath string, v interface{}) error {
	url := c.getURL(apiPath)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	res, statusCode, err := c.request(request)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("get on %s, status: %d", url, statusCode)
	}

	err = json.Unmarshal(res, &v)
	if err != nil {
		return err
	}

	return nil
}

// post performs an HTTP POST and returns a response message.
func (c *SimpleRESTClient) post(apiPath string, body []byte) (response []byte, err error) {
	url := c.getURL(apiPath)

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	response, statusCode, err := c.request(request)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return response, fmt.Errorf("post on %s, status: %d", url, statusCode)
	}
	return response, nil
}

// put performs an HTTP PUT and returns a response message
func (c *SimpleRESTClient) put(apiPath string, body []byte) (response []byte, code int, err error) {
	url := c.getURL(apiPath)

	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, 0, err
	}

	response, statusCode, err := c.request(request)
	if err != nil {
		return nil, statusCode, err
	}

	if statusCode != http.StatusOK {
		return response, statusCode, fmt.Errorf("post on %s, status: %d", url, statusCode)
	}
	return response, statusCode, nil
}

// delete performs an HTTP DELETE and does not return a response message
func (c *SimpleRESTClient) delete(apiPath string) error {
	url := c.getURL(apiPath)
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, statusCode, err := c.request(request)
	if err != nil {
		return err
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("get on %s, status: %d", url, statusCode)
	}

	return nil
}

// find an item by name at the request path
func findItem(client SimpleRESTClient, path string, nameProperty string, idProperty string, name string) (float64, bool, error) {
	// TODO: This is horrible. There is no way to access the database by name so we have
	// to get all the databases and find the UID
	var v interface{}
	err := client.get(path, &v)
	if err != nil {
		return 0, false, fmt.Errorf("cannot get list at %s: %w", path, err)
	}
	var uid float64
	found := false
	for _, item := range v.([]interface{}) {
		m := item.(map[string]interface{})
		if m[nameProperty].(string) == name {
			uid = m[idProperty].(float64)
			found = true
			break
		}
	}

	return uid, found, nil

}

// findDatabase translates from a database name to a cluster internal identifier (UID)
func findDatabase(client SimpleRESTClient, databaseName string) (float64, bool, error) {
	return findItem(client, "/v1/bdbs", "name", "uid", databaseName)
}

// findRole translates from a role name to a cluster internal identifier (UID)
func findRole(client SimpleRESTClient, roleName string) (float64, string, bool, error) {
	// TODO: This is horrible. There is no way to access the database by name so we have
	// to get all the databases and find the UID
	var v interface{}
	err := client.get("/v1/roles", &v)
	if err != nil {
		return 0, "", false, fmt.Errorf("cannot get role list: %w", err)
	}
	var uid float64
	var management string
	found := false
	for _, item := range v.([]interface{}) {
		role := item.(map[string]interface{})
		if role["name"].(string) == roleName {
			uid = role["uid"].(float64)
			management = role["management"].(string)
			found = true
			break
		}
	}

	return uid, management, found, nil

}

// findUser translates from a username to a cluster internal identifier (UID)
func findACL(client SimpleRESTClient, name string) (float64, bool, error) {
	return findItem(client, "/v1/redis_acls", "name", "uid", name)
}

// Verify interface is implemented
var _ dbplugin.Database = (*RedisEnterpriseDB)(nil)

// Our database datastructure only holds the credentials. We have no connection
// to maintain as we're just manipulating the cluster via the REST API.
type RedisEnterpriseDB struct {
	config           config
	logger           hclog.Logger
	client           *sdk.Client
	simpleClient     *SimpleRESTClient
	generateUsername func(string, string) (string, error)
}

func New() (dbplugin.Database, error) {
	// This is normally created for the plugin in plugin.Serve, but dbplugin.Serve doesn't pass into the dbplugin.Database
	// https://github.com/hashicorp/vault/issues/6566
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	generateUsername := func(displayName string, roleName string) (string, error) {
		return credsutil.GenerateUsername(
			credsutil.DisplayName(displayName, 50),
			credsutil.RoleName(roleName, 50),
			credsutil.MaxLength(256),
			credsutil.ToLower(),
		)
	}
	client := sdk.NewClient()

	simpleClient := SimpleRESTClient{
		RoundTripper: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	db := newRedis(logger, client, &simpleClient, generateUsername)
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func newRedis(logger hclog.Logger, client *sdk.Client, simpleClient *SimpleRESTClient, generateUsername func(string, string) (string, error)) *RedisEnterpriseDB {
	return &RedisEnterpriseDB{
		logger:           logger,
		generateUsername: generateUsername,
		client:           client,
		simpleClient:     simpleClient,
	}
}

// SecretVaults returns the configuration information with the password masked
func (r *RedisEnterpriseDB) secretValues() map[string]string {

	// mask secret values in the configuration
	return map[string]string{
		r.config.Password: "[password]",
	}
}

// Initialize copies the configuration information and does a GET on /v1/cluster
// to ensure the cluster is reachable
func (r *RedisEnterpriseDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {

	r.logger.Info("initialising plugin", "version", version.Version, "commit", version.GitCommit)

	if err := mapstructure.WeakDecode(req.Config, &r.config); err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	// Ensure we have the required fields
	if r.config.Url == "" {
		return dbplugin.InitializeResponse{}, errors.New("url is required")
	}
	if r.config.Username == "" {
		return dbplugin.InitializeResponse{}, errors.New("username is required")
	}
	if r.config.Password == "" {
		return dbplugin.InitializeResponse{}, errors.New("password is required")
	}
	// Check optional fields
	if !r.config.hasDatabase() && r.config.hasFeature("acl_only") {
		return dbplugin.InitializeResponse{}, errors.New("the acl_only feature cannot be enabled if there is no database specified")
	}

	r.client.Initialise(r.config.Url, r.config.Username, r.config.Password)
	r.simpleClient.Initialise(r.config.Url, r.config.Username, r.config.Password)

	// Verify the connection to the database if requested.
	if req.VerifyConnection {
		_, err := r.client.GetCluster(ctx)
		if err != nil {
			return dbplugin.InitializeResponse{}, fmt.Errorf("could not verify connection to cluster: %w", err)
		}

		if r.config.hasDatabase() {
			_, err := r.client.FindDatabaseByName(ctx, r.config.Database)
			if err != nil {
				return dbplugin.InitializeResponse{}, fmt.Errorf("could not verify connection to cluster: %w", err)
			}
		}
	}

	response := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	return response, nil
}

const updateRolePermissionsRetryLimit = 30

// Updates the roles_permissions on a bdb with a retry loop.
func updateRolePermissions(client SimpleRESTClient, dbid float64, rolesPermissions []interface{}) error {
	// Update the database
	update_bdb_roles_permissions := map[string]interface{}{
		"roles_permissions": rolesPermissions,
	}
	update_bdb_roles_permissions_body, err := json.Marshal(update_bdb_roles_permissions)
	if err != nil {
		return fmt.Errorf("cannot marshal update database role_permission request: %w", err)
	}
	//fmt.Println(string(update_bdb_roles_permissions_body))

	success := false
	// Retry loop - up to 500ms * limit
	for i := 0; !success && i < updateRolePermissionsRetryLimit; i++ {
		error_response, statusCode, err := client.put(fmt.Sprintf("/v1/bdbs/%.0f", dbid), update_bdb_roles_permissions_body)
		// An HTTP 409 can be return if the database is busy (e.g., with a previous
		// configuration change). So, we pause and retry.
		if statusCode == http.StatusConflict {
			time.Sleep(500 * time.Millisecond)
		} else if err != nil {
			return fmt.Errorf("cannot update database %.0f roles_permissions: %w\n%s", dbid, err, string(error_response))
		} else {
			success = true
		}
	}

	if !success {
		return fmt.Errorf("cannot update database %.0f roles_permissions - too many retries after conflicts (409)", dbid)
	}

	return nil

}

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

// DeleteUser removes a user from the cluster entirely
func (r *RedisEnterpriseDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
	user, err := r.client.FindUserByName(ctx, req.Username)

	if err != nil {
		if _, ok := err.(*sdk.UserNotFoundError); ok {
			// If the user is not found, they may have been deleted manually. We'll assume
			// this is okay and return successfully.
			return dbplugin.DeleteUserResponse{}, nil
		}
		return dbplugin.DeleteUserResponse{}, err
	}

	r.logger.Debug("delete user", "username", req.Username, "uid", user.UID)

	if err := r.client.DeleteUser(ctx, user.UID); err != nil {
		return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot delete user %s: %w", req.Username, err)
	}

	if r.config.hasDatabase() {
		client := r.simpleClient
		// If we have a database we need to there may be a generated
		// role. If we find the generated role by name, we must also delete
		// the generated role binding

		// Find the role id of the potentially generated role
		role := r.config.Database + "-" + req.Username
		rid, _, generatedRole, err := findRole(*client, role)
		if err != nil {
			return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot get roles: %w", err)
		}

		// If we found a role with the name, it was generated by this plugin
		if generatedRole {

			// We must:
			// 1. Retrieve the DB and role ids
			// 2. Find the role binding in roles_permissions in the DB definition
			// 4. Remove the role binding
			// 3. Delete the role

			// Get the database id
			dbid, found, err := findDatabase(*client, r.config.Database)
			if err != nil {
				return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot get databases: %s", err)
			}
			if !found {
				return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot find database: %s", r.config.Database)
			}

			// Get the database information
			var v interface{}
			err = client.get(fmt.Sprintf("/v1/bdbs/%.0f", dbid), &v)
			if err != nil {
				return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot get database info: %s", err)
			}

			// Find the role binding to ACL
			rolesPermissions, found := v.(map[string]interface{})["roles_permissions"].([]interface{})
			if !found {
				return dbplugin.DeleteUserResponse{}, fmt.Errorf("database information has no 'roles_permissions': %s", r.config.Database)
			}
			found_acl := false
			var position int
			for index, value := range rolesPermissions {
				binding := value.(map[string]interface{})
				brole, found := binding["role_uid"]
				if !found {
					continue
				}
				if rid == brole {
					position = index
					found_acl = true
					break
				}
			}

			// If there is a role binding, we must remove the target role
			if found_acl {

				// Remove the binding
				rolesPermissions = append(rolesPermissions[:position], rolesPermissions[position+1:]...)

				// Update the database
				err = updateRolePermissions(*client, dbid, rolesPermissions)
				if err != nil {

					// Attempt to delete the generated role - we know this may fail
					err = client.delete(fmt.Sprintf("/v1/roles/%.0f", rid))
					return dbplugin.DeleteUserResponse{}, fmt.Errorf("user deleted but role and role binding cannot be removed - cannot update role_permissions in database %s: %s", r.config.Database, err)
				}

			}

			// Delete the generated role
			err = client.delete(fmt.Sprintf("/v1/roles/%.0f", rid))
			if err != nil {
				return dbplugin.DeleteUserResponse{}, fmt.Errorf("cannot delete role (%s,%.0f): %s", role, rid, err)
			}

		}

	}
	return dbplugin.DeleteUserResponse{}, nil
}

func (r *RedisEnterpriseDB) Type() (string, error) {
	return redisEnterpriseTypeName, nil
}

func (r *RedisEnterpriseDB) Close() error {
	return r.client.Close()
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

type config struct {
	Features string `mapstructure:"features,omitempty"`
	Database string `mapstructure:"database,omitempty"`
	Username string `mapstructure:"username,omitempty"`
	Password string `mapstructure:"password,omitempty"`
	Url      string `mapstructure:"url,omitempty"`
}

func (c config) hasDatabase() bool {
	return c.Database != ""
}

func (c config) hasFeature(name string) bool {
	if c.Features == "" {
		return false
	}

	for _, value := range strings.Split(c.Features, ",") {
		if value == name {
			return true
		}
	}

	return false
}
