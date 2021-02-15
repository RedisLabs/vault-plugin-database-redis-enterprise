package plugin

import (
	"context"
	"os"
	"testing"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/stretchr/testify/mock"
)

func TestMain(m *testing.M) {
	jsonLogging = false
	os.Exit(m.Run())
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
