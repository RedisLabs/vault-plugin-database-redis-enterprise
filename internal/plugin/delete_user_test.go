package plugin

import (
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/recorder"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestRedisEnterpriseDB_DeleteUser_With_database(t *testing.T) {

	record(t, "DeleteUser_With_database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, false, recorder)

		createReq := dbplugin.NewUserRequest{
			UsernameConfig: dbplugin.UsernameMetadata{
				DisplayName: "tester_del",
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
		assertUserDoesNotExists(t, recorder, url, username, password, userResponse.Username)
	})
}

func TestRedisEnterpriseDB_DeleteUser_Without_database(t *testing.T) {

	record(t, "DeleteUser_Without_database", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, "", false, recorder)

		createReq := dbplugin.NewUserRequest{
			UsernameConfig: dbplugin.UsernameMetadata{
				DisplayName: "tester_del_without_db",
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
		assertUserDoesNotExists(t, recorder, url, username, password, userResponse.Username)
	})
}

func TestRedisEnterpriseDB_DeleteUser_ACLUser(t *testing.T) {

	record(t, "DeleteUser_ACLUser", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, true, recorder)

		createReq := dbplugin.NewUserRequest{
			UsernameConfig: dbplugin.UsernameMetadata{
				DisplayName: "tester_acl_del_with_db",
				RoleName:    "test",
			},
			Statements: dbplugin.Statements{
				Commands: []string{`{"acl":"Not Dangerous"}`},
			},
			Password:   "testing",
			Expiration: time.Now().Add(time.Minute),
		}

		userResponse := dbtesting.AssertNewUser(t, db, createReq)

		role := findRoleForUser(t, recorder, url, username, password, userResponse.Username)

		deleteReq := dbplugin.DeleteUserRequest{
			Username: userResponse.Username,
		}

		dbtesting.AssertDeleteUser(t, db, deleteReq)
		assertUserDoesNotExists(t, recorder, url, username, password, userResponse.Username)
		assertRoleDoesNotExists(t, recorder, url, username, password, role.Name)

		// Verify that the plugin can handle multiple calls to delete a user, in case the user is already deleted
		dbtesting.AssertDeleteUser(t, db, deleteReq)
	})
}
