package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// This REST client just handles the raw requests with JSON and nothing more.
type SimpleRESTClient struct {
	BaseURL      string
	Username     string
	Password     string
	RoundTripper http.RoundTripper
}

// The timeout for the REST client requests.
const timeout = 60

// getURL computes the URL path relative to the base URL and returns it as a string
func (c *SimpleRESTClient) getURL(apiPath string) string {
	return fmt.Sprintf("%s/%s", c.BaseURL, apiPath)
}

func (c *SimpleRESTClient) Initialise(url string, username string, password string) {
	c.BaseURL = strings.TrimSuffix(url, "/")
	c.Username = username
	c.Password = password
}

// request performs an HTTP(S) request, adding various options like authentication. The
// response is return as a tuple that includes the body of the response message and
// status code.
func (c *SimpleRESTClient) request(req *http.Request) (responseBytes []byte, statusCode int, err error) {
	req.SetBasicAuth(c.Username, c.Password)
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	httpClient := http.Client{Timeout: timeout * time.Second, Transport: c.RoundTripper}

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
func (c *SimpleRESTClient) get(apiPath string, v interface{}) error {
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
		return fmt.Errorf("get on %s, status: %d", url, statusCode)
	}

	err = json.Unmarshal(res, &v)
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
		return fmt.Errorf("get on %s, status: %d", url, statusCode)
	}

	return nil
}

// find an item by name at the request path
func findItem(client SimpleRESTClient, path string, nameProperty string, idProperty string, name string) (float64, bool, error) {
	// TODO: This is horrible. There is no way to access the database by name so we have
	// to get all the databases and find the UID
	var v interface{}
	err := client.get(path, &v)
	if err != nil {
		return 0, false, fmt.Errorf("cannot get list at %s: %w", path, err)
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
func findDatabase(client SimpleRESTClient, databaseName string) (float64, bool, error) {
	return findItem(client, "/v1/bdbs", "name", "uid", databaseName)
}

// findRole translates from a role name to a cluster internal identifier (UID)
func findRole(client SimpleRESTClient, roleName string) (float64, string, bool, error) {
	// TODO: This is horrible. There is no way to access the database by name so we have
	// to get all the databases and find the UID
	var v interface{}
	err := client.get("/v1/roles", &v)
	if err != nil {
		return 0, "", false, fmt.Errorf("cannot get role list: %w", err)
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
func findACL(client SimpleRESTClient, name string) (float64, bool, error) {
	return findItem(client, "/v1/redis_acls", "name", "uid", name)
}

const updateRolePermissionsRetryLimit = 30

// Updates the roles_permissions on a bdb with a retry loop.
func updateRolePermissions(client SimpleRESTClient, dbid float64, rolesPermissions []interface{}) error {
	// Update the database
	update_bdb_roles_permissions := map[string]interface{}{
		"roles_permissions": rolesPermissions,
	}
	update_bdb_roles_permissions_body, err := json.Marshal(update_bdb_roles_permissions)
	if err != nil {
		return fmt.Errorf("cannot marshal update database role_permission request: %w", err)
	}
	//fmt.Println(string(update_bdb_roles_permissions_body))

	success := false
	// Retry loop - up to 500ms * limit
	for i := 0; !success && i < updateRolePermissionsRetryLimit; i++ {
		error_response, statusCode, err := client.put(fmt.Sprintf("/v1/bdbs/%.0f", dbid), update_bdb_roles_permissions_body)
		// An HTTP 409 can be return if the database is busy (e.g., with a previous
		// configuration change). So, we pause and retry.
		if statusCode == http.StatusConflict {
			time.Sleep(500 * time.Millisecond)
		} else if err != nil {
			return fmt.Errorf("cannot update database %.0f roles_permissions: %w\n%s", dbid, err, string(error_response))
		} else {
			success = true
		}
	}

	if !success {
		return fmt.Errorf("cannot update database %.0f roles_permissions - too many retries after conflicts (409)", dbid)
	}

	return nil

}
