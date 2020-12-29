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

type SimpleRESTClient struct {
	BaseURL  string
	Username string
	Password string
}

const timeout = 60


func (c *SimpleRESTClient) getURL(apiPath string) string {
	return fmt.Sprintf("%s/%s", c.BaseURL, apiPath)
}

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

func inList(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}



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

func (c *SimpleRESTClient) put(apiPath string, body []byte) (response []byte, err error) {
	url := c.getURL(apiPath)

	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
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



// Verify interface is implemented
var _ dbplugin.Database = (*RedisEnterpriseDB)(nil)

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

func (redb *RedisEnterpriseDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {
   for _, fieldName := range []string{"username", "password", "url"} {
		raw, ok := req.Config[fieldName]
		if !ok {
			return dbplugin.InitializeResponse{}, fmt.Errorf(`%q is required.`, fieldName)
		}
		if _, ok := raw.(string); !ok {
			return dbplugin.InitializeResponse{}, fmt.Errorf(`%q must be a string value`, fieldName)
		}
   }

   redb.Config = req.Config

   if req.VerifyConnection {
      client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}
      var v interface{}
      err := client.get("/v1/cluster",v)
      if err != nil {
   		return dbplugin.InitializeResponse{}, fmt.Errorf("Could not verify connection to cluster: %s", err)
   	}
   }


   response := dbplugin.InitializeResponse{
		Config: req.Config,
	}

   return response, nil
}

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
      return dbplugin.NewUserResponse{}, errors.New("Cannot parse JSON for db roles.")
   }

   m := v.(map[string]interface{})
   role := m["role"].(string)
   uuid, err := newUUID4()
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot generate UUID: %s", err)
   }
   username := "vault-" + req.UsernameConfig.RoleName + "-" + uuid
   fmt.Printf("role: %s\n",role)
   fmt.Printf("username: %s\n",username)

   create_user := map[string]interface{} {
      "name": username,
      "password" : req.Password,
      "role" : role,
      "email_alerts": false,
      "auth_method": "regular",
   }
   create_user_body, err := json.Marshal(create_user)
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot marshal create user request: %s", err)
   }

   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   _, err = client.post("/v1/users",create_user_body)
   if err != nil {
      return dbplugin.NewUserResponse{}, fmt.Errorf("Cannot create user: %s", err)
   }

   return dbplugin.NewUserResponse{Username: username}, nil
}

func findUser(client SimpleRESTClient,username string) (string,bool,error) {
   // TODO: This is horrible. There is no way to access the user by name so we have
   // to get all the users and find the UID
   var v interface{}
   err := client.get("/v1/users",&v)
   if err != nil {
      return "", false, fmt.Errorf("Cannot get user list: %s", err)
   }
   var uid string
   found := false
   for _, item := range v.([]interface{}) {
      user := item.(map[string]interface{})
      if user["name"].(string) == username {
         uid = fmt.Sprintf("%.0f",user["uid"])
         found = true
         break
      }
   }

   return uid, found, nil

}

func (redb *RedisEnterpriseDB) UpdateUser(ctx context.Context, req dbplugin.UpdateUserRequest) (dbplugin.UpdateUserResponse, error)  {
   if req.Password == nil {
      return dbplugin.UpdateUserResponse{}, nil
   }

   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   uid, found, err := findUser(client,req.Username)

   if err != nil {
      return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot get user list: %s", err)
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

   fmt.Printf("Change password for user (%s,%s)\n",req.Username,uid)
   _, err = client.put("/v1/users/"+uid,change_password_body)
   if err != nil {
		return dbplugin.UpdateUserResponse{}, fmt.Errorf("Cannot change user password: %s", err)
	}
   return dbplugin.UpdateUserResponse{}, nil
}

func (redb *RedisEnterpriseDB) DeleteUser(ctx context.Context, req dbplugin.DeleteUserRequest) (dbplugin.DeleteUserResponse, error) {
   client := SimpleRESTClient{BaseURL: strings.TrimSuffix(redb.Config["url"].(string),"/"), Username: redb.Config["username"].(string), Password: redb.Config["password"].(string)}

   uid, found, err := findUser(client,req.Username)

   if err != nil {
      return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot get user list: %s", err)
   }
   if !found {
      return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot find user: %s", req.Username)
   }

   fmt.Printf("Delete user (%s,%s)\n",req.Username,uid)
   err = client.delete("/v1/users/"+uid)
   if err != nil {
		return dbplugin.DeleteUserResponse{}, fmt.Errorf("Cannot change user password: %s", err)
	}

   return dbplugin.DeleteUserResponse{}, nil
}

func (redb *RedisEnterpriseDB) Type() (string, error) {
   return redisEnterpriseTypeName, nil
}

func (redb *RedisEnterpriseDB) Close() (err error) {
   return nil
}
