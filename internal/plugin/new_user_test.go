package plugin

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestRedisEnterpriseDB_NewUser_Without_Database(t *testing.T) {

	record(t, "NewUser_Without_Database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, "", false, recorder)

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

		assertUserExists(t, recorder, url, username, password, res.Username)

		teardownUserFromDatabase(t, recorder, db, res.Username)

	})

}

func TestRedisEnterpriseDB_NewUser_With_Database(t *testing.T) {

	record(t, "NewUser_With_Database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, false, recorder)

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

		assertUserExists(t, recorder, url, username, password, res.Username)

		teardownUserFromDatabase(t, recorder, db, res.Username)
	})
}

func TestRedisEnterpriseDB_NewUser_roleVerifiesACL(t *testing.T) {

	record(t, "NewUser_roleVerifiesACL", func(t *testing.T, recorder *recorder.Recorder) {

		roleName := "DB Member"

		plugin := setupRedisEnterpriseDB(t, database, false, recorder)

		acl := findACLForRole(t, recorder, url, username, password, roleName)

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

		assertUserExists(t, recorder, url, username, password, res.Username)

		assertUserInRole(t, recorder, url, username, password, res.Username, roleName)

		teardownUserFromDatabase(t, recorder, plugin, res.Username)
	})
}

func TestRedisEnterpriseDB_NewUser_rejectsRoleWithDifferentACL(t *testing.T) {

	record(t, "NewUser_rejectsRoleWithDifferentACL", func(t *testing.T, recorder *recorder.Recorder) {

		roleName := "DB Member"

		plugin := setupRedisEnterpriseDB(t, database, false, recorder)

		acl := findACLForRole(t, recorder, url, username, password, roleName)

		altACL := findAlternativeACL(t, recorder, url, username, password, acl.UID)

		createReq := newUserRequest(roleName, altACL.Name)

		_, err := plugin.NewUser(context.Background(), createReq)

		assert.Error(t, err, "Failed to reject a role with the wrong ACL")

	})
}

func TestRedisEnterpriseDB_NewUser_With_Database_With_ACL(t *testing.T) {

	record(t, "NewUser_With_Database_With_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, true, recorder)

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

		assertUserExists(t, recorder, url, username, password, res.Username)

		assertUserHasACL(t, recorder, url, username, password, database, res.Username, aclName)

		teardownUserFromDatabase(t, recorder, db, res.Username)
	})

}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_Cluster(t *testing.T) {

	record(t, "NewUser_Detect_Errors_Cluster", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, "", false, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}} {

			t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
				createReq := newUserRequest(spec[0], spec[1])

				_, err := db.NewUser(context.Background(), createReq)
				assert.Errorf(t, err, "Failed to detect NewUser (cluster) error with role (%s) and acl (%s)", spec[0], spec[1])
			})
		}
	})
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_Without_ACL(t *testing.T) {

	record(t, "NewUser_Detect_Errors_With_Database_Without_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, false, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}, {"garbage", ""}} {

			t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
				createReq := newUserRequest(spec[0], spec[1])

				_, err := db.NewUser(context.Background(), createReq)
				assert.Errorf(t, err, "Failed to detect NewUser (database, no acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
			})
		}
	})
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_With_ACL(t *testing.T) {

	record(t, "NewUser_Detect_Errors_With_Database_With_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, true, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "garbage"}} {

			t.Run(fmt.Sprintf("%v", spec), func(t *testing.T) {
				createReq := newUserRequest(spec[0], spec[1])

				_, err := db.NewUser(context.Background(), createReq)
				assert.Errorf(t, err, "Failed to detect NewUser (database, acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
			})
		}
	})
}

func TestRedisEnterpriseDB_NewUser_createUserWithAclFailureRollsBackCorrectly(t *testing.T) {

	client := &mockSdk{}
	subject := newRedis(hclog.New(&hclog.LoggerOptions{Level: hclog.Trace}),
		client,
		func(displayName string, roleName string) (string, error) {
			return displayName + roleName, nil
		})
	subject.config = config{
		Database: "mocked",
		Features: "acl_only",
	}

	expectedError := errors.New("nope")
	embeddedError := errors.New("failed")

	ctx := context.TODO()

	client.On("FindACLByName", ctx, "expected").Return(&sdk.ACL{UID: 3}, nil)
	client.On("CreateRole", ctx, sdk.CreateRole{
		Name:       "mocked-test-user",
		Management: "db_member",
	}).Return(sdk.Role{UID: 4}, nil)
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
	client.On("CreateUser", ctx, sdk.CreateUser{
		Name:        "test-user",
		Password:    "1234",
		Roles:       []int{4},
		EmailAlerts: false,
		AuthMethod:  "regular",
	}).Return(sdk.User{}, expectedError)
	client.On("DeleteRole", ctx, 4).Return(embeddedError)

	_, err := subject.NewUser(ctx, dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "test-",
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
	subject := newRedis(hclog.New(&hclog.LoggerOptions{Level: hclog.Trace}),
		client,
		func(displayName string, roleName string) (string, error) {
			return displayName + roleName, nil
		})
	subject.config = config{
		Database: "mocked",
		Features: "acl_only",
	}

	expectedError := errors.New("broken")
	embeddedError := errors.New("went wrong")

	ctx := context.TODO()

	client.On("FindACLByName", ctx, "expected").Return(&sdk.ACL{UID: 3}, nil)
	client.On("CreateRole", ctx, sdk.CreateRole{
		Name:       "mocked-test-user",
		Management: "db_member",
	}).Return(sdk.Role{UID: 4}, nil)
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
			DisplayName: "test-",
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
