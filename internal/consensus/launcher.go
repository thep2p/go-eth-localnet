package consensus

// Launcher creates and configures CL client instances.
//
// Different CL implementations (Prysm, Lighthouse, etc.) provide their own
// Launcher implementations. The launcher pattern allows consistent client
// creation across different implementations.
type Launcher interface {
	// Launch creates and starts a new CL client with the given configuration.
	// Returns an error if the client cannot be launched.
	Launch(cfg Config) (Client, error)

	// Name returns the name of this launcher (e.g., "prysm", "lighthouse").
	Name() string

	// ValidateConfig checks if the configuration is valid for this launcher.
	// Returns an error describing any configuration problems.
	ValidateConfig(cfg Config) error
}

// LaunchOption modifies CL client configuration before launch.
//
// LaunchOptions provide a flexible way to configure clients without
// requiring all configuration to be specified upfront.
type LaunchOption func(*Config)

// WithValidatorKeys configures validator keys for block production.
//
// The keys parameter should contain validator private keys in a format
// understood by the specific CL implementation (typically BLS12-381 keys).
// This option is intended for testing only.
func WithValidatorKeys(keys []string) LaunchOption {
	return func(cfg *Config) {
		cfg.ValidatorKeys = keys
	}
}

// WithCheckpointSync enables checkpoint sync from a trusted source.
//
// Checkpoint sync allows the CL client to start from a recent finalized
// checkpoint rather than syncing from genesis, significantly reducing
// initial sync time.
func WithCheckpointSync(url string) LaunchOption {
	return func(cfg *Config) {
		cfg.CheckpointSyncURL = url
	}
}

// WithBootnodes configures bootstrap nodes for P2P discovery.
//
// Bootnodes are initial peers that help the client discover other nodes
// in the network. The bootnodes parameter should contain ENR addresses.
func WithBootnodes(bootnodes []string) LaunchOption {
	return func(cfg *Config) {
		cfg.Bootnodes = bootnodes
	}
}

// WithStaticPeers configures static peer connections.
//
// Static peers are maintained as persistent connections, useful for
// ensuring connectivity in private networks or small testnets.
func WithStaticPeers(peers []string) LaunchOption {
	return func(cfg *Config) {
		cfg.StaticPeers = peers
	}
}

// WithEngineEndpoint configures the Engine API endpoint.
//
// The endpoint parameter should be the full URL of the execution layer's
// Engine API (e.g., "http://localhost:8551").
func WithEngineEndpoint(endpoint string) LaunchOption {
	return func(cfg *Config) {
		cfg.EngineEndpoint = endpoint
	}
}

// WithJWTSecret configures the JWT secret for Engine API authentication.
//
// The secret must match the JWT secret configured on the execution layer
// for authentication to succeed.
func WithJWTSecret(secret []byte) LaunchOption {
	return func(cfg *Config) {
		cfg.JWTSecret = secret
	}
}
