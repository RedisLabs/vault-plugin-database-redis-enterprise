package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testServer(t *testing.T, path string, method string, expectedUsername string, expectedPassword string, f func(w http.ResponseWriter, r *http.Request)) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !assert.Equal(t, path, r.URL.Path) || !assert.Equal(t, method, r.Method) {
			http.Error(w, "invalid request", http.StatusInternalServerError)
			return
		}

		var ok bool
		username, password, ok := r.BasicAuth()
		if !assert.True(t, ok) || !assert.Equal(t, expectedUsername, username) || !assert.Equal(t, expectedPassword, password) {
			http.Error(w, "basic auth failure", http.StatusInternalServerError)
			return
		}

		f(w, r)
	}))

	t.Cleanup(func() {
		server.Close()
	})

	return server.URL
}
