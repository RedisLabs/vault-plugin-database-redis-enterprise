package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/assert"

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

func TestRedisEnterpriseDB_NewUser_With_Database_With_ACL(t *testing.T) {

	record(t, "NewUser_With_Database_With_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, true, recorder)

		createRequest := dbplugin.NewUserRequest{
			UsernameConfig: dbplugin.UsernameMetadata{
				DisplayName: "tester_new_with_db_with_acl",
				RoleName:    "test",
			},
			Statements: dbplugin.Statements{
				Commands: []string{`{"acl":"Not Dangerous"}`},
			},
			Password:   "testing",
			Expiration: time.Now().Add(time.Minute),
		}

		res := dbtesting.AssertNewUser(t, db, createRequest)

		assertUserExists(t, recorder, url, username, password, res.Username)

		teardownUserFromDatabase(t, recorder, db, res.Username)
	})

}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_Cluster(t *testing.T) {

	record(t, "NewUser_Detect_Errors_Cluster", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, "", false, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}} {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (cluster) error with role (%s) and acl (%s)", spec[0], spec[1])
		}
	})
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_Without_ACL(t *testing.T) {

	record(t, "NewUser_Detect_Errors_With_Database_Without_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, false, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "Not Dangerous"}, {"garbage", ""}} {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (database, no acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		}
	})
}

func TestRedisEnterpriseDB_NewUser_Detect_Errors_With_Database_With_ACL(t *testing.T) {

	record(t, "NewUser_Detect_Errors_With_Database_With_ACL", func(t *testing.T, recorder *recorder.Recorder) {

		db := setupRedisEnterpriseDB(t, database, true, recorder)

		for _, spec := range [][]string{{"", ""}, {"", "garbage"}} {
			createReq := newUserRequest(spec[0], spec[1])

			_, err := db.NewUser(context.Background(), createReq)
			assert.Errorf(t, err, "Failed to detect NewUser (database, acl_only) error with role (%s) and acl (%s)", spec[0], spec[1])
		}
	})
}
