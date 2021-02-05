package vault_plugin_database_redisenterprise

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestPlugin(t *testing.T) {

	url := os.Getenv("RS_API_URL")
	username := os.Getenv("RS_USERNAME")
	password := os.Getenv("RS_PASSWORD")
	db := os.Getenv("RS_DB")

	t.Run("Initialize", func(t *testing.T) { testRedisEnterpriseDBInitialize(t, url, username, password, "") })
	t.Run("Initialize - database", func(t *testing.T) { testRedisEnterpriseDBInitialize(t, url, username, password, db) })
	t.Run("Initialize - errors", func(t *testing.T) { testRedisEnterpriseDBInitializeErrors(t, url, username, password, db) })
	t.Run("NewUser", func(t *testing.T) { testRedisEnterpriseNewUser(t, url, username, password, "", false) })
	t.Run("NewUser - database", func(t *testing.T) { testRedisEnterpriseNewUser(t, url, username, password, db, false) })
	t.Run("NewUser - acl", func(t *testing.T) { testRedisEnterpriseNewUser(t, url, username, password, db, true) })
	t.Run("NewUser - errors", func(t *testing.T) { testRedisEnterpriseNewUserErrors(t, url, username, password, db) })
	t.Run("UpdateUser change password", func(t *testing.T) { testRedisEnterpriseUpdateUserChangePassword(t, url, username, password) })
	t.Run("DeleteUser", func(t *testing.T) { testRedisEnterpriseDeleteUser(t, url, username, password, "") })
	t.Run("DeleteUser - database", func(t *testing.T) { testRedisEnterpriseDeleteUser(t, url, username, password, db) })

}

const context_timeout = 2 * time.Second

func initDatabaseRequest(url string, username string, password string, database string, enableACL bool) dbplugin.InitializeRequest {
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

	req := dbplugin.InitializeRequest{
		Config:           config,
		VerifyConnection: true,
	}

	return req
}

func initDatabase(t *testing.T, url string, username string, password string, database string, enableACL bool) *RedisEnterpriseDB {
	req := initDatabaseRequest(url, username, password, database, enableACL)

	db := newRedis(hclog.Default())

	dbtesting.AssertInitialize(t, db, req)

	return db
}

func testRedisEnterpriseDBInitialize(t *testing.T, url string, username string, password string, database string) {
	t.Log("Testing initialize - no TLS")

	db := initDatabase(t, url, username, password, database, false)

	err := db.Close()
	if err != nil {
		t.Fatalf("Cannot close database: %s", err)
	}

}

func testRedisEnterpriseDBInitializeErrors(t *testing.T, url string, username string, password string, database string) {
	t.Log("Testing initialize - errors")

	t.Helper()

	req := initDatabaseRequest(url, username, password, "", true)

	ctx, cancel := context.WithTimeout(context.Background(), context_timeout)
	defer cancel()

	db := newRedis(hclog.Default())

	_, err := db.Initialize(ctx, req)
	if err == nil {
		t.Fatal("Failed to detect no database with acl_only")
	}
}

func testRedisEnterpriseNewUser(t *testing.T, url string, username string, password string, database string, useACL bool) {
	t.Log("Testing new user")

	db := initDatabase(t, url, username, password, database, useACL)

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"DB Member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	if useACL {
		createReq.Statements.Commands = []string{`{"acl":"Not Dangerous"}`}
	}

	res := dbtesting.AssertNewUser(t, db, createReq)

	assertUserExists(t, url, username, password, res.Username)

}

func assertUserExists(t *testing.T, url string, username string, password string, generatedUser string) {
	client := sdk.NewClient(url, username, password)
	users, err := client.ListUsers(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range users {
		if u.Name == generatedUser {
			return
		}
	}

	t.Errorf("unable to find user %s", generatedUser)
}

func assertUserDoesNotExists(t *testing.T, url string, username string, password string, generatedUser string) {
	client := sdk.NewClient(url, username, password)
	users, err := client.ListUsers(context.TODO())
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range users {
		if u.Name == generatedUser {
			t.Errorf("found user %s", generatedUser)
		}
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

func testRedisEnterpriseNewUserErrors(t *testing.T, url string, username string, password string, database string) {
	t.Log("Testing new user - errors")
	t.Helper()

	cluster := initDatabase(t, url, username, password, "", false)

	for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}} {
		createReq := newUserRequest(spec[0], spec[1])

		ctx, cancel := context.WithTimeout(context.Background(), context_timeout)
		defer cancel()

		_, err := cluster.NewUser(ctx, createReq)
		if err == nil {
			t.Fatalf("Failed to detect NewUser (cluster) error with role (%s) and acl (%s)", spec[0], spec[1])
		}

	}

	db := initDatabase(t, url, username, password, database, false)

	for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}, {"garbage", ""}} {
		createReq := newUserRequest(spec[0], spec[1])

		ctx, cancel := context.WithTimeout(context.Background(), context_timeout)
		defer cancel()

		_, err := db.NewUser(ctx, createReq)
		if err == nil {
			t.Fatalf("Failed to detect NewUser (database, no acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		}

	}

	db_allow_acl := initDatabase(t, url, username, password, database, true)

	for _, spec := range [][]string{{"", ""}, {"", "garbage"}} {
		createReq := newUserRequest(spec[0], spec[1])

		ctx, cancel := context.WithTimeout(context.Background(), context_timeout)
		defer cancel()

		_, err := db_allow_acl.NewUser(ctx, createReq)
		if err == nil {
			t.Fatalf("Failed to detect NewUser (database, acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		}

	}

}

func testRedisEnterpriseUpdateUserChangePassword(t *testing.T, url string, username string, password string) {
	t.Log("Testing user password change")

	db := initDatabase(t, url, username, password, "", false)

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"DB Member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	userResponse := dbtesting.AssertNewUser(t, db, createReq)

	client := sdk.NewClient(url, username, password)
	beforeUpdate, err := client.FindUserByName(context.TODO(), userResponse.Username)
	if err != nil {
		t.Fatal(err)
	}

	// Wait a bit so the password updated date will be different
	time.Sleep(2 * time.Second)

	updateReq := dbplugin.UpdateUserRequest{
		Username: userResponse.Username,
		Password: &dbplugin.ChangePassword{
			NewPassword: "xyzzyxyzzy",
		},
	}

	dbtesting.AssertUpdateUser(t, db, updateReq)

	afterUpdate, err := client.FindUserByName(context.TODO(), userResponse.Username)
	if err != nil {
		t.Fatal(err)
	}

	if beforeUpdate.PasswordIssueDate == afterUpdate.PasswordIssueDate {
		t.Errorf("password hasn't been updated")
	}

}

func testRedisEnterpriseDeleteUser(t *testing.T, url string, username string, password string, database string) {
	t.Log("Testing delete user")

	db := initDatabase(t, url, username, password, database, false)

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"DB Member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

	userResponse := dbtesting.AssertNewUser(t, db, createReq)

	deleteReq := dbplugin.DeleteUserRequest{
		Username: userResponse.Username,
	}

	dbtesting.AssertDeleteUser(t, db, deleteReq)

	assertUserDoesNotExists(t, url, username, password, userResponse.Username)

}
