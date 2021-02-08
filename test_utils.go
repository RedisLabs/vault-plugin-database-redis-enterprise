package vault_plugin_database_redisenterprise

import (
	"crypto/tls"
	"fmt"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"strconv"
	"testing"
)

var(
	disbaleFixtures = getEnvAsBool("RS_DISABLE_FIXTURES", false)
)

func record(t *testing.T, fixture string, f func(*testing.T, *recorder.Recorder)) {

	cassetteName := fmt.Sprintf("fixtures/%s", fixture)

	fixtureMode := recorder.ModeReplaying
	if disbaleFixtures {
		fixtureMode = recorder.ModeDisabled
	}

	r, err := recorder.NewAsMode(cassetteName, fixtureMode, &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	})
	require.NoError(t, err)

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

func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

func getEnvAsBool(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}

	return defaultVal
}
