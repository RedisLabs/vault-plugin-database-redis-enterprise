package plugin

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/sdk"
	"github.com/RedisLabs/vault-plugin-database-redisenterprise/internal/version"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/sdk/database/dbplugin/v5"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
	"github.com/mitchellh/mapstructure"
)

const redisEnterpriseTypeName = "redisenterprise"

// Verify interface is implemented
var _ dbplugin.Database = (*RedisEnterpriseDB)(nil)

// Our database datastructure only holds the credentials. We have no connection
// to maintain as we're just manipulating the cluster via the REST API.
type RedisEnterpriseDB struct {
	config           config
	logger           hclog.Logger
	client           *sdk.Client
	simpleClient     *SimpleRESTClient
	generateUsername func(string, string) (string, error)
}

func New() (dbplugin.Database, error) {
	// This is normally created for the plugin in plugin.Serve, but dbplugin.Serve doesn't pass into the dbplugin.Database
	// https://github.com/hashicorp/vault/issues/6566
	logger := hclog.New(&hclog.LoggerOptions{
		Level:      hclog.Trace,
		Output:     os.Stderr,
		JSONFormat: true,
	})

	generateUsername := func(displayName string, roleName string) (string, error) {
		return credsutil.GenerateUsername(
			credsutil.DisplayName(displayName, 50),
			credsutil.RoleName(roleName, 50),
			credsutil.MaxLength(256),
			credsutil.ToLower(),
		)
	}
	client := sdk.NewClient()

	simpleClient := SimpleRESTClient{
		RoundTripper: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	db := newRedis(logger, client, &simpleClient, generateUsername)
	dbType := dbplugin.NewDatabaseErrorSanitizerMiddleware(db, db.secretValues)
	return dbType, nil
}

func newRedis(logger hclog.Logger, client *sdk.Client, simpleClient *SimpleRESTClient, generateUsername func(string, string) (string, error)) *RedisEnterpriseDB {
	return &RedisEnterpriseDB{
		logger:           logger,
		generateUsername: generateUsername,
		client:           client,
		simpleClient:     simpleClient,
	}
}

// SecretVaults returns the configuration information with the password masked
func (r *RedisEnterpriseDB) secretValues() map[string]string {

	// mask secret values in the configuration
	return map[string]string{
		r.config.Password: "[password]",
	}
}

// Initialize copies the configuration information and does a GET on /v1/cluster
// to ensure the cluster is reachable
func (r *RedisEnterpriseDB) Initialize(ctx context.Context, req dbplugin.InitializeRequest) (dbplugin.InitializeResponse, error) {

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
	if !r.config.hasDatabase() && r.config.hasFeature("acl_only") {
		return dbplugin.InitializeResponse{}, errors.New("the acl_only feature cannot be enabled if there is no database specified")
	}

	r.client.Initialise(r.config.Url, r.config.Username, r.config.Password)
	r.simpleClient.Initialise(r.config.Url, r.config.Username, r.config.Password)

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

func (r *RedisEnterpriseDB) Type() (string, error) {
	return redisEnterpriseTypeName, nil
}

func (r *RedisEnterpriseDB) Close() error {
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
