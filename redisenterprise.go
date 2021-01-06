package vault_plugin_database_redisenterprise

import (
   "bytes"
   "context"
   "errors"
   "fmt"
   "crypto/tls"
   "crypto/rand"
   "net/http"
   "encoding/json"
   "time"
   "io"
   "io/ioutil"
   "strings"

   dbplugin "github.com/hashicorp/vault/sdk/database/dbplugin/v5"
   //recClient "github.com/RedisLabs/vault-plugin-database-redisenterprise/pkg/redis-enterprise-client"
)

const redisEnterpriseTypeName = "redisenterprise"

// newUUID generates a random UUID according to RFC 4122
// https://play.golang.org/p/4FkNSiUDMg
func newUUID4() (string, error) {
   uuid := make([]byte, 16)
   n, err := io.ReadFull(rand.Reader, uuid)
   if n != len(uuid) || err != nil {
      return "", err
   }
   // variant bits; see section 4.1.1
   uuid[8] = uuid[8]&^0xc0 | 0x80
   // version 4 (pseudo-random); see section 4.1.3
   uuid[6] = uuid[6]&^0xf0 | 0x40
   return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

// This REST client just handles the raw requests with JSON and nothing more.
type SimpleRESTClient struct {
   BaseURL  string
   Username string
   Password string
}

// The timeout for the REST client requests.
const timeout = 60

// getURL computes the URL path relative to the base URL and returns it as a string
func (c *SimpleRESTClient) getURL(apiPath string) string {
   return fmt.Sprintf("%s/%s", c.BaseURL, apiPath)
}

// request performs an HTTP(S) request, adding various options like authentication. The
// response is return as a tuple that includes the body of the response message and
// status code.
func (c *SimpleRESTClient) request(req *http.Request) (responseBytes []byte, statusCode int, err error) {
   req.SetBasicAuth(c.Username, c.Password)
   req.Header.Add("Content-Type", "application/json;charset=utf-8")
   tr := &http.Transport{
      TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
   }
   httpClient := http.Client{Timeout: timeout * time.Second, Transport: tr}

   response, err := httpClient.Do(req)
   if err != nil {
      return nil, -1, err
   }

   responseBytes, err = ioutil.ReadAll(response.Body)
   defer response.Body.Close()
   if err != nil {
      return nil, -1, err
   }
   return responseBytes, response.StatusCode, nil
}

// get performs an HTTP get and returns a JSON response message
func (c *SimpleRESTClient) get(apiPath string,v interface{}) error {
   url := c.getURL(apiPath)
   request, err := http.NewRequest("GET", url, nil)
   if err != nil {
      return err
   }

   res, statusCode, err := c.request(request)
   if err != nil {
      return err
   }

   if statusCode != http.StatusOK {
      return fmt.Errorf("Get on %s, status: %d", url, statusCode)
   }


   err = json.Unmarshal([]byte(res), &v)
   if err != nil {
      return err
   }

   return nil
}

// post performs an HTTP POST and returns a response message.
func (c *SimpleRESTClient) post(apiPath string, body []byte) (response []byte, err error) {
   url := c.getURL(apiPath)

   request, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
   if err != nil {
      return nil, err
   }

   response, statusCode, err := c.request(request)
   if err != nil {
      return nil, err
   }

   if statusCode != http.StatusOK {
      return response, fmt.Errorf("post on %s, status: %d", url, statusCode)
   }
   return response, nil
}

// put performs an HTTP PUT and returns a response message
func (c *SimpleRESTClient) put(apiPath string, body []byte) (response []byte, code int, err error) {
   url := c.getURL(apiPath)

   request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
   if err != nil {
      return nil, 0, err
   }

   response, statusCode, err := c.request(request)
   if err != nil {
      return nil, statusCode, err
   }

   if statusCode != http.StatusOK {
      return response, statusCode, fmt.Errorf("post on %s, status: %d", url, statusCode)
   }
   return response, statusCode, nil
}

// delete performs an HTTP DELETE and does not return a response message
func (c *SimpleRESTClient) delete(apiPath string) error {
   url := c.getURL(apiPath)
   request, err := http.NewRequest("DELETE", url, nil)
   if err != nil {
      return err
   }

   _, statusCode, err := c.request(request)
   if err != nil {
      return err
   }

   if statusCode != http.StatusOK {
      return fmt.Errorf("Get on %s, status: %d", url, statusCode)
   }

   return nil
}


// find an item by name at the request path
func findItem(client SimpleRESTClient,path string,nameProperty string, idProperty string,name string) (float64,bool,error) {
   // TODO: This is horrible. There is no way to access the database by name so we have
   // to get all the databases and find the UID
   var v interface{}
   err := client.get(path,&v)
   if err != nil {
      return 0, false, fmt.Errorf("Cannot get list at %s: %s", path, err)
   }
   var uid float64
   found := false
   for _, item := range v.([]interface{}) {
      m := item.(map[string]interface{})
      if m[nameProperty].(string) == name {
         uid = m[idProperty].(float64)
         found = true
         break
      }
   }

   return uid, found, nil

}

// findDatabase translates from a database name to a cluster internal identifier (UID)
func findDatabase(client SimpleRESTClient,databaseName string) (float64,bool,error) {

   return findItem(client,"/v1/bdbs","name","uid",databaseName)
   // TODO: This is horrible. There is no way to access the database by name so we have
   // to get all the databases and find the UID
   // var v interface{}
   // err := client.get("/v1/bdbs",&v)
   // if err != nil {
   //    return 0, false, fmt.Errorf("Cannot get database list: %s", err)
   // }
   // var uid float64
   // found := false
   // for _, item := range v.([]interface{}) {
   //    db := item.(map[string]interface{})
   //    if db["name"].(string) == databaseName {
   //       uid = db["uid"].(float64)
   //       found = true
   //       break
   //    }
   // }
   //
   // return uid, found, nil

}

// findDatabase translates from a database name to a cluster internal identifier (UID)
func findRole(client SimpleRESTClient,roleName string) (float64,string,bool,error) {
   // TODO: This is horrible. There is no way to access the database by name so we have
   // to get all the databases and find the UID
   var v interface{}
   err := client.get("/v1/roles",&v)
   if err != nil {
      return 0, "",false, fmt.Errorf("Cannot get role list: %s", err)
   }
   var uid float64
   var management string
   found := false
   for _, item := range v.([]interface{}) {
      role := item.(map[string]interface{})
      if role["name"].(string) == roleName {
         uid = role["uid"].(float64)
         management = role["management"].(string)
         found = true
         break
      }
   }

   return uid, management, found, nil

}


// findUser translates from a username to a cluster internal identifier (UID)
func findUser(client SimpleRESTClient,username string) (float64,bool,error) {
   return findItem(client,"/v1/users","name","uid",username)
   // TODO: This is horrible. There is no way to access the user by name so we have
   // to get all the users and find the UID
   // var v interface{}
   // err := client.get("/v1/users",&v)
   // if err != nil {
   //    return 0, false, fmt.Errorf("Cannot get user list: %s", err)
   // }
   // var uid float64
   // found := false
   // for _, item := range v.([]interface{}) {
   //    user := item.(map[string]interface{})
   //    if user["name"].(string) == username {
   //       uid = user["uid"].(float64)
   //       found = true
   //       break
   //    }
   // }
   //
   // return uid, found, nil

}

// findUser translates from a username to a cluster internal identifier (UID)
func findACL(client SimpleRESTClient,name string) (float64,bool,error) {
   return findItem(client,"/v1/redis_acls","name","uid",name)
}



// Verify interface is implemented
var _ dbplugin.Database = (*RedisEnterpriseDB)(nil)

// Our database datastructure only holds the credentials. We have no connection
// to maintain as we're just manipulating the cluster via the REST API.
type RedisEnterpriseDB struct {
   Config map[string] interface{}
}

func New() (interface{}, error) {
   db := new()
   dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.SecretValues)
   return dbType, nil
}

func new() *RedisEnterpriseDB {
   return &RedisEnterpriseDB{}
}

// SecretVaults returns the configuration information with the password masked
func (redb *RedisEnterpriseDB) SecretValues() map[string]string {

   // mask secret values in the configuration
   replacements := make(map[string]string)
   for _, secretName := range []string{"password"} {
      vIfc, found := redb.Config[secretName]
      if !found {
         continue
      }
      secretVal, ok := vIfc.(string)
      if !ok {
         continue
      }
      replacements[secretVal] = "[" + secretName + "]"
   }
   return replacements
}

// Initialize copies the configuration information and does a GET on /v1/cluster
// to ensure the cluster is reachable
func (redb *RedisEnterpriseDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {

   redb.Config = make(map[string]interface{})

   // Ensure we have the required fields
   for _, fieldName := range []string{"username", "password", "url"} {
      raw, ok := req.Config[fieldName]
      if !ok {
         return dbplugin.InitializeResponse{}, fmt.Errorf(`%q is required.`, fieldName)
      }
      if _, ok := raw.(string); !ok {
         return dbplugin.InitializeResponse{}, fmt.Errorf(`%q must be a string value`, fieldName)
      }
      redb.Config[fieldName] = raw
   }
   // Check optional fields
   for _, fieldName := range []string{"database"} {
      raw, ok := req.Config[fieldName]
      if !ok {
         continue
      }
      if _, ok := raw.(string); !ok {
         return dbplugin.InitializeResponse{}, fmt.Errorf(`%q must be a string value`, fieldName)
      }
      redb.Config[fieldName] = raw
   }


   // Verify the connection to the database if requested.
   if req.VerifyConnection {
      client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}
      var v interface{}
      err := client.get("/v1/cluster",v)
      if err != nil {
         return dbplugin.InitializeResponse{}, fmt.Errorf("Could not verify connection to cluster: %s", err)
      }
      database, ok := req.Config["database"].(string)

      if ok {
         _, found, err := findDatabase(client,database)
         if err != nil {
            return dbplugin.InitializeResponse{}, fmt.Errorf("Could not verify connection to cluster: %s", err)
         }
         if !found {
            return dbplugin.InitializeResponse{}, fmt.Errorf("Database does not exist: %s", database)
         }
      }
   }


   response := dbplugin.InitializeResponse{
      Config: req.Config,
   }

   return response, nil
}

const updateRolePermissionsRetryLimit = 30

func updateRolePermissions(client SimpleRESTClient,dbid float64, rolesPermissions []interface{}) error {
   // Update the database
   update_bdb_roles_permissions := map[string]interface{} {
      "roles_permissions" : rolesPermissions,
   }
   update_bdb_roles_permissions_body, err := json.Marshal(update_bdb_roles_permissions)
   if err != nil {
      return fmt.Errorf("Cannot marshal update database role_permission request: %s", err)
   }
   //fmt.Println(string(update_bdb_roles_permissions_body))

   success := false
   for i:=0; !success && i<updateRolePermissionsRetryLimit; i++ {
      error_response, statusCode, err := client.put(fmt.Sprintf("/v1/bdbs/%.0f",dbid),update_bdb_roles_permissions_body)
      if statusCode == http.StatusConflict {
         time.Sleep(500 * time.Millisecond)
      } else if err != nil {
         return fmt.Errorf("Cannot update database %.0f roles_permissions: %s\n%s", dbid, err, string(error_response))
      } else {
         success = true
      }
   }

   if !success {
      return fmt.Errorf("Cannot update database %.0f roles_permissions - too many retries after conflicts (409).",dbid)
   }

   return nil

}

// NewUser creates a new user and authentication credentials in the cluster.
// The statement is required to be JSON with the structure:
// {
//    "role" : "role_name"
// }
// The role name is must exist the cluster before the user can be created.
func (redb *RedisEnterpriseDB) NewUser(ctx context.Context, req dbplugin.NewUserRequest) (dbplugin.NewUserResponse, error)  {
   fmt.Printf("display: %s\n",req.UsernameConfig.DisplayName)
   fmt.Printf("role: %s\n",req.UsernameConfig.RoleName)
   for _, statement := range req.Statements.Commands {
      fmt.Printf("statement: %s\n",statement)
   }

   if len(req.Statements.Commands) < 1 {
      return dbplugin.NewUserResponse{}, errors.New("No creation statements were provided. The groups are not defined.")
   }

   var v interface {}
   err := json.Unmarshal([]byte(req.Statements.Commands[0]), &v)

   if err != nil {
      return dbplugin.NewUserResponse{}, errors.New("Cannot parse JSON for db role.")
   }

   m := v.(map[string]interface{})
   role, hasRole := m["role"].(string)
   acl, hasACL := m["acl"].(string)
   if !hasRole && !hasACL {
      return dbplugin.NewUserResponse{}, fmt.Errorf("No 'role' or 'acl' in creation statement for %s", req.UsernameConfig.RoleName)
   }
   uuid, err := newUUID4()
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot generate UUID: %s", err)
   }
   username := "vault-" + req.UsernameConfig.RoleName + "-" + uuid
   if hasRole {
      fmt.Printf("role: %s\n",role)
   }
   if hasACL {
      fmt.Printf("acl: %s\n",acl)
   }
   fmt.Printf("username: %s\n",username)

   database, hasDatabase := redb.Config["database"].(string)

   if !hasDatabase && hasACL {
      return dbplugin.NewUserResponse{}, fmt.Errorf("ACL cannot be used when the database has not been specified for %s", req.UsernameConfig.RoleName)
   }

   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   var create_user map[string]interface{}

   var rid float64
   var role_management string
   var aid float64

   if hasRole {
      // get the role id
      var found bool
      rid, role_management, found, err = findRole(client,role)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot get roles: %s", err)
      }
      if !found {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot find role: %s", role)
      }
   }

   if hasACL {
      // get the ACL id
      var found bool
      aid, found, err = findACL(client,acl)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot get acls: %s", err)
      }
      if !found {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot find acl: %s", acl)
      }
      role_management = "db_member"
   }

   if hasDatabase {

      // If we have a database we need to:
      // 1. Retrieve the DB and role ids
      // 2. Find the role binding in roles_permissions in the DB definition
      // 3. Create a new role for the user
      // 4. Bind the new role to the same ACL in the database

      // Get the database id
      dbid, found, err := findDatabase(client,database)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot get databases: %s", err)
      }
      if !found {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot find database: %s", database)
      }

      // Get the database information
      var v interface{}
      err = client.get(fmt.Sprintf("/v1/bdbs/%.0f",dbid),&v)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot get database info: %s", err)
      }

      // Find the role binding to ACL
      rolesPermissions, found := v.(map[string]interface{})["roles_permissions"].([]interface{})
      if !found {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Database information has no 'roles_permissions': %s",database)
      }

      // {
      //    role_permission_serialized, err := json.Marshal(rolesPermissions)
      //    if err != nil {
      //       return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot marshal roles_permissions : %s", err)
      //    }
      //    fmt.Println("Before:")
      //    fmt.Println(string(role_permission_serialized))
      // }

      if !hasACL {
         found_acl := false
         for _, value := range rolesPermissions {
            binding := value.(map[string]interface{})
            brole, found := binding["role_uid"]
            if !found {
               continue
            }
            if rid == brole {
               aid, found = binding["redis_acl_uid"].(float64)
               if !found {
                  continue
               }
               found_acl = true
               break
            }
         }
         if !found_acl {
            return dbplugin.NewUserResponse{}, fmt.Errorf("Database %s has no binding for role %s",database,role)
         }
      }

      // Create a new role
      vault_role := database + "-" + username
      create_role := map[string]interface{} {
         "name": vault_role,
         "management": role_management,
      }
      create_role_body, err := json.Marshal(create_role)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot marshal create role request: %s", err)
      }

      create_role_response_raw, err := client.post("/v1/roles",create_role_body)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot create role: %s", err)
      }

      var create_role_response interface{}
      err = json.Unmarshal([]byte(create_role_response_raw), &create_role_response)
      if err != nil {
         return dbplugin.NewUserResponse{}, err
      }

      // Add the new binding to the same ACL
      new_role_id := create_role_response.(map[string]interface{})["uid"].(float64)

      new_binding := map[string]interface{} {
         "role_uid" : new_role_id,
         "redis_acl_uid" : aid,
      }
      rolesPermissions = append(rolesPermissions,new_binding)

      // Update the database
      err = updateRolePermissions(client,dbid,rolesPermissions)
      if err != nil {
         return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot update role_permissions in database %s: %s", database, err)
      }

      rid = new_role_id

   }

   create_user = map[string]interface{} {
      "name": username,
      "password" : req.Password,
      "role_uids" : []float64{rid},
      "email_alerts": false,
      "auth_method": "regular",
   }
   create_user_body, err := json.Marshal(create_user)
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot marshal create user request: %s", err)
   }

   _, err = client.post("/v1/users",create_user_body)
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot create user: %s", err)
   }

   return dbplugin.NewUserResponse{Username: username}, nil
}

// UpdateUser changes a user's password
func (redb *RedisEnterpriseDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error)  {
   if req.Password == nil {
      return dbplugin.UpdateUserResponse{}, nil
   }

   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   uid, found, err := findUser(client,req.Username)

   if err != nil {
      return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot get users: %s", err)
   }
   if !found {
      return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot find user: %s", req.Username)
   }

   change_password := map[string]interface{} {
      "password" : req.Password.NewPassword,
   }

   change_password_body, err := json.Marshal(change_password)
   if err != nil {
      return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot marshal change user password request: %s", err)
   }

   fmt.Printf("Change password for user (%s,%.0f)\n",req.Username,uid)
   _, _, err = client.put(fmt.Sprintf("/v1/users/%.0f",uid),change_password_body)
   if err != nil {
      return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot change user password: %s", err)
   }
   return dbplugin.UpdateUserResponse{}, nil
}

// DeleteUser removes a user from the cluster entirely
func (redb *RedisEnterpriseDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   uid, found, err := findUser(client,req.Username)

   if err != nil {
      return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot get user list: %s", err)
   }
   if !found {
      // If the user is not found, they may have been deleted manually. We'll assume
      // this is okay and return successfully.
      return dbplugin.DeleteUserResponse{}, nil
   }

   fmt.Printf("Delete user (%s,%.0f)\n",req.Username,uid)
   err = client.delete(fmt.Sprintf("/v1/users/%.0f",uid))
   if err != nil {
      return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot delete user %s: %s",req.Username, err)
   }

   database, hasDatabase := redb.Config["database"].(string)

   if hasDatabase {

      // If we have a database we need to:
      // 1. Retrieve the DB and role ids
      // 2. Find the role binding in roles_permissions in the DB definition
      // 4. Remove the role binding
      // 3. Delete the role

      role := database + "-" + req.Username

      // Get the database id
      dbid, found, err := findDatabase(client,database)
      if err != nil {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot get databases: %s", err)
      }
      if !found {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot find database: %s", database)
      }

      // get the role id
      rid, _, found, err := findRole(client,role)
      if err != nil {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot get roles: %s", err)
      }
      if !found {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot find role: %s", role)
      }

      // Get the database information
      var v interface{}
      err = client.get(fmt.Sprintf("/v1/bdbs/%.0f",dbid),&v)
      if err != nil {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot get database info: %s", err)
      }

      // Find the role binding to ACL
      rolesPermissions, found := v.(map[string]interface{})["roles_permissions"].([]interface{})
      if !found {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Database information has no 'roles_permissions': %s",database)
      }
      found_acl := false
      var position int
      for index, value := range rolesPermissions {
         binding := value.(map[string]interface{})
         brole, found := binding["role_uid"]
         if !found {
            continue
         }
         if rid == brole {
            position = index
            found_acl = true
            break
         }
      }
      if found_acl {

         // Remove the binding
         rolesPermissions = append(rolesPermissions[:position], rolesPermissions[position+1:]...)

         // Update the database
         err = updateRolePermissions(client,dbid,rolesPermissions)
         if err != nil {

            // Attempt to delete the generated role - we know this may fail
            err = client.delete(fmt.Sprintf("/v1/roles/%.0f",rid))
            return dbplugin.DeleteUserResponse{}, fmt.Errorf("User deleted but role and role binding cannot be removed - cannot update role_permissions in database %s: %s", database, err)
         }

      }

      // Delete the generated role
      err = client.delete(fmt.Sprintf("/v1/roles/%.0f",rid))
      if err != nil {
         return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot delete role (%s,%.0f): %s",role,rid,err)
      }

   }
   return dbplugin.DeleteUserResponse{}, nil
}

func (redb *RedisEnterpriseDB) Type() (string, error) {
   return redisEnterpriseTypeName, nil
}

func (redb *RedisEnterpriseDB) Close() (err error) {
   return nil
}
