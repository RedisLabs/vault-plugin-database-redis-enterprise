package sdk

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_UpdateDatabaseWithRetry_retries(t *testing.T) {
	counter := 0
	var body []byte
	var actualUsername, actualPassword string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bdbs/3" {
			http.Error(w, fmt.Sprintf("invalid path %s", r.URL.Path), http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusInternalServerError)
			return
		}
		var ok bool
		actualUsername, actualPassword, ok = r.BasicAuth()
		if !ok {
			http.Error(w, "basic auth failure", http.StatusInternalServerError)
			return
		}

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
	}))

	defer server.Close()

	expectedUsername := "expected"
	expectedPassword := "Password"

	subject := &Client{
		url:      server.URL,
		username: expectedUsername,
		password: expectedPassword,
		Client:   http.DefaultClient,
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

	assert.Equal(t, expectedUsername, actualUsername)
	assert.Equal(t, expectedPassword, actualPassword)
	assert.JSONEq(t, `{"roles_permissions": [{"role_uid": 1, "redis_acl_uid": 2}]}`, string(body))
}

func TestClient_UpdateDatabaseWithRetry_givesUpOnError(t *testing.T) {
	counter := 0
	var body []byte
	var actualUsername, actualPassword string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/bdbs/3" {
			http.Error(w, fmt.Sprintf("invalid path %s", r.URL.Path), http.StatusInternalServerError)
			return
		}
		if r.Method != http.MethodPut {
			http.Error(w, fmt.Sprintf("invalid method %s", r.Method), http.StatusInternalServerError)
			return
		}
		var ok bool
		actualUsername, actualPassword, ok = r.BasicAuth()
		if !ok {
			http.Error(w, "basic auth failure", http.StatusInternalServerError)
			return
		}

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
	}))

	defer server.Close()

	subject := &Client{
		url:      server.URL,
		username: "expected",
		password: "Password",
		Client:   http.DefaultClient,
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
	assert.Equal(t, "expected", actualUsername)
	assert.Equal(t, "Password", actualPassword)
	assert.JSONEq(t, `{"roles_permissions": [{"role_uid": 1, "redis_acl_uid": 2}]}`, string(body))
}
