package plugin

import (
	"context"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"
	"time"

	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"
)

func TestRedisEnterpriseDB_UpdateUser_With_New_Password(t *testing.T) {
	db := setupRedisEnterpriseDB(t, "", false)

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

	client := sdk.NewClient(hclog.Default())
	client.Initialise(url, username, password)

	beforeUpdate, err := client.FindUserByName(context.TODO(), userResponse.Username)
	require.NoError(t, err)

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
	require.NoError(t, err)

	assert.NotEqual(t, beforeUpdate.PasswordIssueDate, afterUpdate.PasswordIssueDate)

	teardownUserFromDatabase(t, db, userResponse.Username)
}

func TestRedisEnterpriseDB_UpdateUser_findUserByEmail(t *testing.T) {
	// The root user for the plugin will typically have been configured using the email address
	// so the plugin needs to support updating the password of a user based on the email address rather than their name
	db := setupRedisEnterpriseDB(t, database, false)

	client := sdk.NewClient(hclog.Default())
	client.Initialise(url, username, password)

	email := "updateStaticUser@example.test"

	role, err := client.FindRoleByName(context.Background(), "DB Member")
	require.NoError(t, err)

	user, err := client.CreateUser(context.Background(), sdk.CreateUser{
		Name:        t.Name(),
		Email:       email,
		Password:    "Password123!",
		Roles:       []int{role.UID},
		EmailAlerts: false,
		AuthMethod:  "regular",
	})
	require.NoError(t, err)

	// Wait a bit so the password updated date will be different
	time.Sleep(2 * time.Second)

	updateReq := dbplugin.UpdateUserRequest{
		Username: email,
		Password: &dbplugin.ChangePassword{
			NewPassword: "xyzzyxyzzy",
		},
	}

	dbtesting.AssertUpdateUser(t, db, updateReq)

	afterUpdate, err := client.GetUser(context.Background(), user.UID)
	require.NoError(t, err)

	assert.NotEqual(t, user.PasswordIssueDate, afterUpdate.PasswordIssueDate)

	teardownUserFromDatabase(t, db, email)
}
