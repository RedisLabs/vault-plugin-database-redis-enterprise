package vault_plugin_database_redisenterprise

import (
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)


func TestRedisEnterpriseDB_DeleteUser(t *testing.T) {

	db := setupRedisEnterpriseDB(t, database, enableACL)

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
	assertUserDoesNotExists(t, url, username, password, userResponse.Username)
}

func TestRedisEnterpriseDB_DeleteUser_Without_database(t *testing.T) {

	database := ""

	db := setupRedisEnterpriseDB(t, database, enableACL)

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
	assertUserDoesNotExists(t, url, username, password, userResponse.Username)
}
