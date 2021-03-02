package sdk

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_UpdateDatabaseWithRetry_retries(t *testing.T) {
	counter := 0
	var body []byte
	username := "expected"
	password := "Password"

	url := testServer(t, "/v1/bdbs/3", http.MethodPut, username, password, func(w http.ResponseWriter, r *http.Request) {
		counter++
		if counter < 3 {
			http.Error(w, "try again", http.StatusConflict)
			return
		}

		var err error
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("can't read body %s", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	subject := &Client{
		url:      url,
		username: username,
		password: password,
		client:   http.DefaultClient,
	}

	err := subject.UpdateDatabaseWithRetry(context.TODO(), 3, UpdateDatabase{
		RolePermissions: []RolePermission{
			{
				RoleUID: 1,
				ACLUID:  2,
			},
		},
	})

	require.NoError(t, err)
	assert.JSONEq(t, `{"roles_permissions": [{"role_uid": 1, "redis_acl_uid": 2}]}`, string(body))
}

func TestClient_UpdateDatabaseWithRetry_givesUpOnError(t *testing.T) {
	counter := 0
	var body []byte
	username := "expected"
	password := "Password"

	url := testServer(t, "/v1/bdbs/3", http.MethodPut, username, password, func(w http.ResponseWriter, r *http.Request) {
		counter++
		if counter < 3 {
			http.Error(w, "try again", http.StatusConflict)
			return
		}

		var err error
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("can't read body %s", err), http.StatusInternalServerError)
			return
		}

		http.Error(w, "done", http.StatusTeapot)
	})

	subject := &Client{
		url:      url,
		username: username,
		password: password,
		client:   http.DefaultClient,
	}

	err := subject.UpdateDatabaseWithRetry(context.TODO(), 3, UpdateDatabase{
		RolePermissions: []RolePermission{
			{
				RoleUID: 1,
				ACLUID:  2,
			},
		},
	})

	assert.Equal(t, &HttpError{
		method: "PUT",
		path:   "/v1/bdbs/3",
		status: 418,
		body:   "done",
	}, err)
	assert.JSONEq(t, `{"roles_permissions": [{"role_uid": 1, "redis_acl_uid": 2}]}`, string(body))
}
