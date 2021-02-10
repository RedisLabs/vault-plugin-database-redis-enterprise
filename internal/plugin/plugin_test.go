package plugin

import (
	"context"
	"fmt"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

var (
	url      = os.Getenv("RS_API_URL")
	username = os.Getenv("RS_USERNAME")
	password = os.Getenv("RS_PASSWORD")
	database = os.Getenv("RS_DB")
)

func setupRedisEnterpriseDB(t *testing.T, database string, enableACL bool, recorder *recorder.Recorder) *redisEnterpriseDB {
	t.Helper()

	request := initializeRequest(url, username, password, database, enableACL)

	client := sdk.NewClient()
	client.Client.Transport = recorder

	db := newRedis(hclog.New(&hclog.LoggerOptions{Level: hclog.Trace}),
		client,
		func(displayName string, roleName string) (string, error) {
			return displayName + roleName, nil
		})

	dbtesting.AssertInitialize(t, db, request)
	return db
}

func TestRedisEnterpriseDB_Initialize_Without_Database(t *testing.T) {

	record(t, "Initialize_Without_Database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, "", false, recorder)

		err := db.Close()
		require.NoError(t, err)

	})
}

func TestRedisEnterpriseDB_Initialize_With_Database(t *testing.T) {

	record(t, "Initialize_With_Database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, false, recorder)

		err := db.Close()
		require.NoError(t, err)

	})
}

func TestRedisEnterpriseDB_Initialize_Without_Database_With_ACL(t *testing.T) {

	request := initializeRequest(url, username, password, "", true)
	db := newRedis(hclog.Default(), nil, func(displayName string, roleName string) (string, error) {
		return displayName + roleName, nil
	})

	_, err := db.Initialize(context.Background(), request)
	assert.Error(t, err, "Failed to detect no database with acl_only")
}

func TestRedisEnterpriseDB_Initialize_shouldErrorWithoutURL(t *testing.T) {

	request := initializeRequest("", username, password, database, false)
	db := newRedis(hclog.Default(), nil, func(displayName string, roleName string) (string, error) {
		return displayName + roleName, nil
	})

	_, err := db.Initialize(context.Background(), request)
	assert.Error(t, err, "Failed to detect no URL")
}

func TestRedisEnterpriseDB_Initialize_shouldErrorWithoutUsername(t *testing.T) {

	request := initializeRequest(url, "", password, database, false)
	db := newRedis(hclog.Default(), nil, func(displayName string, roleName string) (string, error) {
		return displayName + roleName, nil
	})

	_, err := db.Initialize(context.Background(), request)
	assert.Error(t, err, "Failed to detect no URL")
}

func TestRedisEnterpriseDB_Initialize_shouldErrorWithoutPassword(t *testing.T) {

	request := initializeRequest(url, username, "", database, false)
	db := newRedis(hclog.Default(), nil, func(displayName string, roleName string) (string, error) {
		return displayName + roleName, nil
	})

	_, err := db.Initialize(context.Background(), request)
	assert.Error(t, err, "Failed to detect no password")
}

func assertUserExists(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, generatedUser string) {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	users, err := client.ListUsers(context.TODO())
	require.NoError(t, err)

	assert.Contains(t, userNames(users), generatedUser)
}

func assertUserInRole(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, generatedUser string, roleName string) {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	user, err := client.FindUserByName(context.TODO(), generatedUser)
	require.NoError(t, err)

	role, err := client.FindRoleByName(context.TODO(), roleName)
	require.NoError(t, err)

	assert.Equal(t, user.Roles, []int{role.UID})
}

func assertUserHasACL(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, database string, generatedUser string, aclName string) {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	user, err := client.FindUserByName(context.TODO(), generatedUser)
	require.NoError(t, err)

	db, err := client.FindDatabaseByName(context.TODO(), database)
	require.NoError(t, err)

	role := db.FindPermissionForRole(user.Roles[0])
	require.NotNil(t, role)

	acl, err := client.GetACL(context.TODO(), role.ACLUID)
	require.NoError(t, err)

	assert.Equal(t, aclName, acl.Name)
}

func findRoleForUser(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, generatedUser string) sdk.Role {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	user, err := client.FindUserByName(context.TODO(), generatedUser)
	require.NoError(t, err)

	require.Len(t, user.Roles, 1)

	role, err := client.GetRole(context.TODO(), user.Roles[0])
	require.NoError(t, err)

	return role
}

func findACLForRole(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, roleName string) sdk.ACL {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	role, err := client.FindRoleByName(context.Background(), roleName)
	require.NoError(t, err)
	db, err := client.FindDatabaseByName(context.Background(), database)
	require.NoError(t, err)
	perm := db.FindPermissionForRole(role.UID)
	require.NotNil(t, perm)
	acl, err := client.GetACL(context.Background(), perm.ACLUID)
	require.NoError(t, err)
	return acl
}

func findAlternativeACL(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, aclId int) sdk.ACL {
	t.Helper()
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	acls, err := client.ListACLs(context.Background())
	require.NoError(t, err)

	for _, acl := range acls {
		if acl.UID != aclId {
			return acl
		}
	}

	require.Fail(t, "Unable to find an alternative ACL")
	return sdk.ACL{}
}

func initializeRequest(url string, username string, password string, database string, enableACL bool) dbplugin.InitializeRequest {
	config := map[string]interface{}{
		"url":      url,
		"username": username,
		"password": password,
	}

	if len(database) > 0 {
		config["database"] = database
	}

	if enableACL {
		config["features"] = "acl_only"
	}

	return dbplugin.InitializeRequest{
		Config:           config,
		VerifyConnection: true,
	}
}

func newUserRequest(role string, acl string) dbplugin.NewUserRequest {
	command := `{}`
	if len(role) > 0 && len(acl) > 0 {
		command = fmt.Sprintf(`{"role":"%s","acl":"%s"}`, role, acl)
	} else if len(role) > 0 {
		command = fmt.Sprintf(`{"role":"%s"}`, role)
	} else if len(acl) > 0 {
		command = fmt.Sprintf(`{"acl":"%s"}`, acl)
	}
	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{command},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}
	return createReq
}

func assertUserDoesNotExists(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, generatedUser string) {
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	users, err := client.ListUsers(context.TODO())
	require.NoError(t, err)

	assert.NotContains(t, userNames(users), generatedUser)
}

func assertRoleDoesNotExists(t *testing.T, recorder *recorder.Recorder, url string, username string, password string, generatedRole string) {
	client := sdk.NewClient()
	client.Client.Transport = recorder
	client.Initialise(url, username, password)

	roles, err := client.ListRoles(context.TODO())
	require.NoError(t, err)

	assert.NotContains(t, roleNames(roles), generatedRole)
}

func teardownUserFromDatabase(t *testing.T, recorder *recorder.Recorder, db *redisEnterpriseDB, generatedUser string) {
	t.Helper()
	dbtesting.AssertDeleteUser(t, db, dbplugin.DeleteUserRequest{
		Username: generatedUser,
	})
	assertUserDoesNotExists(t, recorder, url, username, password, generatedUser)
}

func userNames(users []sdk.User) []string {
	var names []string
	for _, u := range users {
		names = append(names, u.Name)
	}
	return names
}

func roleNames(roles []sdk.Role) []string {
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}
