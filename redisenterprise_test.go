package vault_plugin_database_redisenterprise

import (
   //"context"
   "testing"
   "os"
   "time"

   dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
   dbtesting "github.com/hashicorp/vault/sdk/database/dbplugin/v5/testing"

)

func TestPlugin(t *testing.T) {

   url := os.Getenv("RS_API_URL")
   username := os.Getenv("RS_USERNAME")
   password := os.Getenv("RS_PASSWORD")

   t.Run("Initialize", func(t *testing.T) { testRedisEnterpriseDBInitialize(t,url,username,password) })
   t.Run("NewUser", func(t *testing.T) { testRedisEnterpriseNewUser(t,url,username,password) })
   t.Run("UpdateUser change password", func(t *testing.T) { testRedisEnterpriseUpdateUserChangePassword(t,url,username,password) })
   t.Run("DeleteUser", func(t *testing.T) { testRedisEnterpriseDeleteUser(t,url,username,password) })

}

func initDatabase(t *testing.T, url string, username string, password string) *RedisEnterpriseDB {
   config := map[string]interface{}{
      "url" : url,
      "username" : username,
      "password" : password,
   }

   req := dbplugin.InitializeRequest{
      Config: config,
      VerifyConnection: true,
   }

   db := new()

   dbtesting.AssertInitialize(t, db, req)

   return db
}


func testRedisEnterpriseDBInitialize(t *testing.T, url string, username string, password string) {
   t.Log("Testing initialize - no TLS")

   db := initDatabase(t,url,username,password)

   err := db.Close()
   if err != nil {
      t.Fatalf("Cannot close database: %s",err)
   }

}

func testRedisEnterpriseNewUser(t *testing.T, url string, username string, password string) {
   t.Log("Testing new user")

   db := initDatabase(t,url,username,password)

	createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
		   DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"db_member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

   dbtesting.AssertNewUser(t, db, createReq)


}

func testRedisEnterpriseUpdateUserChangePassword(t *testing.T, url string, username string, password string) {
   t.Log("Testing user password change")

   db := initDatabase(t,url,username,password)

   createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
		   DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"db_member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

   userResponse := dbtesting.AssertNewUser(t, db, createReq)

   updateReq := dbplugin.UpdateUserRequest{
      Username: userResponse.Username,
      Password: &dbplugin.ChangePassword{
			NewPassword: "xyzzyxyzzy",
		},
   }

   dbtesting.AssertUpdateUser(t, db, updateReq)

}

func testRedisEnterpriseDeleteUser(t *testing.T, url string, username string, password string) {
   t.Log("Testing delete user")

   db := initDatabase(t,url,username,password)

   createReq := dbplugin.NewUserRequest{
		UsernameConfig: dbplugin.UsernameMetadata{
		   DisplayName: "tester",
			RoleName:    "test",
		},
		Statements: dbplugin.Statements{
			Commands: []string{`{"role":"db_member"}`},
		},
		Password:   "testing",
		Expiration: time.Now().Add(time.Minute),
	}

   userResponse := dbtesting.AssertNewUser(t, db, createReq)

   deleteReq := dbplugin.DeleteUserRequest{
      Username: userResponse.Username,
   }

   dbtesting.AssertDeleteUser(t, db, deleteReq)

}
