package sdk

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_FindUserByName_useNameFirst(t *testing.T) {
	expectedId := 2
	name := "needle"
	username := "expected"
	password := "Password"

	url := testServer(t, "/v1/users", http.MethodGet, username, password, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `[{"uid":-1,"name":"other","email":%[2]q},{"uid":%[1]d,"name":%[2]q}]`, expectedId, name)
	})

	subject := &Client{
		url:      url,
		username: username,
		password: password,
		client:   http.DefaultClient,
	}

	actual, err := subject.FindUserByName(context.Background(), name)
	require.NoError(t, err)

	assert.Equal(t, expectedId, actual.UID)
}

func TestClient_FindUserByName_fallBackToEmail(t *testing.T) {
	expectedId := 2
	name := "needle"

	username := "expected"
	password := "Password"

	url := testServer(t, "/v1/users", http.MethodGet, username, password, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `[{"uid":-1,"name":"other","email":"foo@example.com"},{"uid":%[1]d,"email":%[2]q}]`, expectedId, name)
	})

	subject := &Client{
		url:      url,
		username: username,
		password: password,
		client:   http.DefaultClient,
	}

	actual, err := subject.FindUserByName(context.Background(), name)
	require.NoError(t, err)

	assert.Equal(t, expectedId, actual.UID)
}
