package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestRedisEnterpriseDB_NewUser_Without_Database(t *testing.T) {
	db := setupRedisEnterpriseDB(t, "", false)

	createRequest := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester_new_without_db",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"DB Member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	res := dbtesting.AssertNewUser(t, db, createRequest)

	assertUserExists(t, url, username, password, res.Username)

	teardownUserFromDatabase(t, db, res.Username)
}

func TestRedisEnterpriseDB_NewUser_With_Database(t *testing.T) {
	db := setupRedisEnterpriseDB(t, database, false)

	createRequest := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester_new_with_db",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"DB Member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	res := dbtesting.AssertNewUser(t, db, createRequest)

	assertUserExists(t, url, username, password, res.Username)

	teardownUserFromDatabase(t, db, res.Username)
}

func TestRedisEnterpriseDB_NewUser_roleVerifiesACL(t *testing.T) {
	roleName := "DB Member"

	plugin := setupRedisEnterpriseDB(t, database, false)

	acl := findACLForRole(t, url, username, password, roleName)

	createRequest := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester_new_role_verifies_acl",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{fmt.Sprintf(`{"role":%q, "acl": %q}`, roleName, acl.Name)},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	res := dbtesting.AssertNewUser(t, plugin, createRequest)

	assertUserExists(t, url, username, password, res.Username)

	assertUserInRole(t, url, username, password, res.Username, roleName)

	teardownUserFromDatabase(t, plugin, res.Username)
}

func TestRedisEnterpriseDB_NewUser_rejectsRoleWithDifferentACL(t *testing.T) {
	roleName := "DB Member"

	plugin := setupRedisEnterpriseDB(t, database, false)

	acl := findACLForRole(t, url, username, password, roleName)

	altACL := findAlternativeACL(t, url, username, password, acl.UID)

	createReq := newUserRequest(roleName, altACL.Name)

	_, err := plugin.NewUser(context.Background(), createReq)

	assert.Error(t, err, "Failed to reject a role with the wrong ACL")
}

func TestRedisEnterpriseDB_NewUser_With_Database_With_ACL(t *testing.T) {
	db := setupRedisEnterpriseDB(t, database, true)

	aclName := "Not Dangerous"

	createRequest := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester_new_with_db_with_acl",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{fmt.Sprintf(`{"acl":%q}`, aclName)},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	res := dbtesting.AssertNewUser(t, db, createRequest)

	assertUserExists(t, url, username, password, res.Username)

	assertUserHasACL(t, url, username, password, database, res.Username, aclName)

	teardownUserFromDatabase(t, db, res.Username)
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_Cluster(t *testing.T) {
	db := setupRedisEnterpriseDB(t, "", false)

	for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}} {

		t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (cluster) error with role (%s) and acl (%s)", spec[0], spec[1])
		})
	}
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_Without_ACL(t *testing.T) {
	db := setupRedisEnterpriseDB(t, database, false)

	for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}, {"garbage", ""}} {

		t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (database, no acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		})
	}
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_With_ACL(t *testing.T) {
	db := setupRedisEnterpriseDB(t, database, true)

	for _, spec := range [][]string{{"", ""}, {"", "garbage"}} {

		t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (database, acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		})
	}
}

func TestRedisEnterpriseDB_NewUser_createUserWithAclFailureRollsBackCorrectly(t *testing.T) {

	client := &mockSdk{}
	subject := newRedis(hclog.New(&hclog.LoggerOptions{Level: hclog.Trace}), client)
	subject.config = config{
		Database: "mocked",
		Features: "acl_only",
	}

	expectedError := errors.New("nope")
	embeddedError := errors.New("failed")

	ctx := context.TODO()

	client.On("FindACLByName", ctx, "expected").Return(&sdk.ACL{UID: 3}, nil)
	client.On("CreateRole", matchesContext(ctx), matchesCreateRole("db_member", "mocked", "test", "user")).Return(sdk.Role{UID: 4}, nil)
	client.On("FindDatabaseByName", ctx, "mocked").Return(sdk.Database{
		UID: 5,
		RolePermissions: []sdk.RolePermission{
			{
				RoleUID: 5,
				ACLUID:  6,
			},
		},
	}, nil)
	client.On("UpdateDatabaseWithRetry", ctx, 5, sdk.UpdateDatabase{
		RolePermissions: []sdk.RolePermission{
			{
				RoleUID: 5,
				ACLUID:  6,
			},
			{
				RoleUID: 4,
				ACLUID:  3,
			},
		},
	}).Return(nil)
	client.On("CreateUser", matchesContext(ctx), matchesCreateUser("test", "user", 4, "1234")).Return(sdk.User{}, expectedError)
	client.On("DeleteRole", ctx, 4).Return(embeddedError)

	_, err := subject.NewUser(ctx, dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "test",
			RoleName:    "user",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"acl": "expected"}`},
		},
		Password: "1234",
	})

	require.Error(t, err)
	assert.Equal(t, multierror.Append(expectedError, embeddedError), err)
}

func TestRedisEnterpriseDB_NewUser_createRoleFailureRollsBackCorrectly(t *testing.T) {

	client := &mockSdk{}
	subject := newRedis(hclog.New(&hclog.LoggerOptions{Level: hclog.Trace}), client)
	subject.config = config{
		Database: "mocked",
		Features: "acl_only",
	}

	expectedError := errors.New("broken")
	embeddedError := errors.New("went wrong")

	ctx := context.TODO()

	client.On("FindACLByName", ctx, "expected").Return(&sdk.ACL{UID: 3}, nil)
	client.On("CreateRole", matchesContext(ctx), matchesCreateRole("db_member", "mocked", "test", "user")).Return(sdk.Role{UID: 4}, nil)
	client.On("FindDatabaseByName", ctx, "mocked").Return(sdk.Database{
		UID: 5,
		RolePermissions: []sdk.RolePermission{
			{
				RoleUID: 5,
				ACLUID:  6,
			},
		},
	}, nil)
	client.On("UpdateDatabaseWithRetry", ctx, 5, sdk.UpdateDatabase{
		RolePermissions: []sdk.RolePermission{
			{
				RoleUID: 5,
				ACLUID:  6,
			},
			{
				RoleUID: 4,
				ACLUID:  3,
			},
		},
	}).Return(expectedError)
	client.On("DeleteRole", ctx, 4).Return(embeddedError)

	_, err := subject.NewUser(ctx, dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "test",
			RoleName:    "user",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"acl": "expected"}`},
		},
		Password: "1234",
	})

	require.Error(t, err)
	assert.Equal(t, multierror.Append(expectedError, embeddedError), err)
}

func matchesContext(ctx context.Context) interface{} {
	return mock.MatchedBy(func(ctx2 context.Context) bool { return ctx2 == ctx })
}

func matchesCreateRole(management string, dbName string, displayName string, roleName string) interface{} {
	return mock.MatchedBy(func(r sdk.CreateRole) bool {
		if r.Management != management {
			return false
		}

		if !strings.HasPrefix(r.Name, dbName) {
			return false
		}

		parts := strings.Split(r.Name, "_")
		if len(parts) < 3 {
			return false
		}
		if parts[1] != displayName || parts[2] != roleName {
			return false
		}
		return true
	})
}

func matchesCreateUser(displayName string, roleName string, roleId int, password string) interface{} {
	return mock.MatchedBy(func(c sdk.CreateUser) bool {
		if c.AuthMethod != "regular" {
			return false
		}

		if c.EmailAlerts {
			return false
		}

		if c.Password != password {
			return false
		}

		if len(c.Roles) != 1 || c.Roles[0] != roleId {
			return false
		}

		parts := strings.Split(c.Name, "_")
		if len(parts) < 2 {
			return false
		}
		if parts[1] != displayName || parts[2] != roleName {
			return false
		}
		return true
	})
}
