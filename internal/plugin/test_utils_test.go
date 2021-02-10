package plugin

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	disableFixtures = getEnvAsBool("RS_DISABLE_FIXTURES", false)
)

func record(t *testing.T, fixture string, f func(*testing.T, *recorder.Recorder)) {

	cassetteName := fmt.Sprintf("fixtures/%s", fixture)

	fixtureMode := recorder.ModeReplaying
	if disableFixtures {
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

var _ sdkClient = &mockSdk{}

type mockSdk struct {
	mock.Mock
}

func (m *mockSdk) Initialise(url string, username string, password string) {
	m.Called(url, username, password)
}

func (m *mockSdk) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSdk) FindACLByName(ctx context.Context, name string) (*sdk.ACL, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*sdk.ACL), args.Error(1)
}

func (m *mockSdk) GetCluster(ctx context.Context) (sdk.Cluster, error) {
	args := m.Called(ctx)
	return args.Get(0).(sdk.Cluster), args.Error(1)
}

func (m *mockSdk) UpdateDatabaseWithRetry(ctx context.Context, id int, update sdk.UpdateDatabase) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *mockSdk) FindDatabaseByName(ctx context.Context, name string) (sdk.Database, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(sdk.Database), args.Error(1)
}

func (m *mockSdk) CreateRole(ctx context.Context, create sdk.CreateRole) (sdk.Role, error) {
	args := m.Called(ctx, create)
	return args.Get(0).(sdk.Role), args.Error(1)
}

func (m *mockSdk) DeleteRole(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSdk) FindRoleByName(ctx context.Context, name string) (sdk.Role, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(sdk.Role), args.Error(1)
}

func (m *mockSdk) CreateUser(ctx context.Context, create sdk.CreateUser) (sdk.User, error) {
	args := m.Called(ctx, create)
	return args.Get(0).(sdk.User), args.Error(1)
}

func (m *mockSdk) UpdateUserPassword(ctx context.Context, id int, update sdk.UpdateUser) error {
	args := m.Called(ctx, id, update)
	return args.Error(0)
}

func (m *mockSdk) DeleteUser(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockSdk) FindUserByName(ctx context.Context, name string) (sdk.User, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(sdk.User), args.Error(1)
}
