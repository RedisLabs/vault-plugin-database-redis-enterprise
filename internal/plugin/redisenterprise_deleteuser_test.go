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
