package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/version"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/mitchellh/mapstructure"
)

const redisEnterpriseTypeName = "redisenterprise"

// Verify interface is implemented
var _ dbplugin.Database = (*redisEnterpriseDB)(nil)

// jsonLogging sets whether the logs should be outputted in JSON format or not.
// This is solely to allow the tests to be able to display messages in a friendly format - Vault needs logging to be in
// JSON format to correctly display the logs of the plugin.
var jsonLogging = true

type redisEnterpriseDB struct {
	config config
	logger hclog.Logger
	client sdkClient

	// databaseRolePermissions is used to attempt to avoid buried writes with multiple updates to the database
	// permissions at the same time, although something may still be updating the database at the same time.
	databaseRolePermissions *sync.Mutex
}

func New() (dbplugin.Database, error) {
	// This is normally created for the plugin in plugin.Serve, but dbplugin.Serve doesn't pass into the dbplugin.Database
	// https://github.com/hashicorp/vault/issues/6566
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: jsonLogging,
	})

	client := sdk.NewClient()

	db := newRedis(logger, client)
	return wrapWithSanitizerMiddleware(db), nil
}

func newRedis(logger hclog.Logger, client sdkClient) *redisEnterpriseDB {
	return &redisEnterpriseDB{
		logger:                  logger,
		client:                  client,
		databaseRolePermissions: &sync.Mutex{},
	}
}

func wrapWithSanitizerMiddleware(db *redisEnterpriseDB) dbplugin.Database {
	return dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
}

// secretVaults returns the configuration information with the password masked
func (r *redisEnterpriseDB) secretValues() map[string]string {

	// mask secret values in the configuration
	return map[string]string{
		r.config.Password: "[password]",
	}
}

// Initialize copies the configuration information and does a GET on /v1/cluster
// to ensure the cluster is reachable
func (r *redisEnterpriseDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {

	r.logger.Info("initialising plugin", "version", version.Version, "commit", version.GitCommit)

	if err := mapstructure.WeakDecode(req.Config, &r.config); err != nil {
		return dbplugin.InitializeResponse{}, err
	}

	// Ensure we have the required fields
	if r.config.Url == "" {
		return dbplugin.InitializeResponse{}, errors.New("url is required")
	}
	if r.config.Username == "" {
		return dbplugin.InitializeResponse{}, errors.New("username is required")
	}
	if r.config.Password == "" {
		return dbplugin.InitializeResponse{}, errors.New("password is required")
	}
	// Check optional fields
	if !r.config.hasDatabase() && r.config.supportAclOnly() {
		return dbplugin.InitializeResponse{}, errors.New("the acl_only feature cannot be enabled if there is no database specified")
	}

	r.client.Initialise(r.config.Url, r.config.Username, r.config.Password)

	// Verify the connection to the database if requested.
	if req.VerifyConnection {
		_, err := r.client.GetCluster(ctx)
		if err != nil {
			return dbplugin.InitializeResponse{}, fmt.Errorf("could not verify connection to cluster: %w", err)
		}

		if r.config.hasDatabase() {
			_, err := r.client.FindDatabaseByName(ctx, r.config.Database)
			if err != nil {
				return dbplugin.InitializeResponse{}, fmt.Errorf("could not verify connection to cluster: %w", err)
			}
		}
	}

	response := dbplugin.InitializeResponse{
		Config: req.Config,
	}

	return response, nil
}

func (r *redisEnterpriseDB) Type() (string, error) {
	return redisEnterpriseTypeName, nil
}

func (r *redisEnterpriseDB) Close() error {
	return r.client.Close()
}

type config struct {
	Features string `mapstructure:"features,omitempty"`
	Database string `mapstructure:"database,omitempty"`
	Username string `mapstructure:"username,omitempty"`
	Password string `mapstructure:"password,omitempty"`
	Url      string `mapstructure:"url,omitempty"`
}

func (c config) hasDatabase() bool {
	return c.Database != ""
}

func (c config) hasFeature(name string) bool {
	if c.Features == "" {
		return false
	}

	for _, value := range strings.Split(c.Features, ",") {
		if value == name {
			return true
		}
	}

	return false
}

func (c config) supportAclOnly() bool {
	return c.hasFeature("acl_only")
}

type sdkClient interface {
	Initialise(url string, username string, password string)
	Close() error
	FindACLByName(ctx context.Context, name string) (*sdk.ACL, error)
	GetCluster(ctx context.Context) (sdk.Cluster, error)
	UpdateDatabaseWithRetry(ctx context.Context, id int, update sdk.UpdateDatabase) error
	FindDatabaseByName(ctx context.Context, name string) (sdk.Database, error)
	CreateRole(ctx context.Context, create sdk.CreateRole) (sdk.Role, error)
	DeleteRole(ctx context.Context, id int) error
	FindRoleByName(ctx context.Context, name string) (sdk.Role, error)
	CreateUser(ctx context.Context, create sdk.CreateUser) (sdk.User, error)
	UpdateUserPassword(ctx context.Context, id int, update sdk.UpdateUser) error
	DeleteUser(ctx context.Context, id int) error
	FindUserByName(ctx context.Context, name string) (sdk.User, error)
}
