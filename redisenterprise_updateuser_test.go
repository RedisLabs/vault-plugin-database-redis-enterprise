package vault_plugin_database_redisenterprise

import (
	"context"
	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestRedisEnterpriseDB_UpdateUser_With_New_Password(t *testing.T) {

	database := ""

	db := setupRedisEnterpriseDB(t, database, enableACL)

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
			DisplayName: "tester_update",
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

	teardownUserFromDatabase(t, db, userResponse.Username)
}
