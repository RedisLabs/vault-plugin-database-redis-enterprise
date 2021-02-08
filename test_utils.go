package vault_plugin_database_redisenterprise

import (
	"crypto/tls"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func record(t *testing.T, fixture string, f func(*testing.T, *recorder.Recorder)) {
	r, err := recorder.New("fixtures/" + fixture)
	require.NoError(t, err)

	r.SetTransport(&http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
	r.AddFilter(func(i *cassette.Interaction) error {
		delete(i.Request.Headers, "Authorization")
		return nil
	})

	defer func() {
		err := r.Stop()
		require.NoError(t, err)
	}()

	f(t, r)
}
