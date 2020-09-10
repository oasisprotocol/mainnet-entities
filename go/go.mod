module github.com/oasisprotocol/mainnet-entities/go

go 1.14

replace (
	// Replace directives from oasis-core, as these don't get propagated.
	github.com/gorilla/websocket => github.com/gorilla/websocket v1.4.2
	github.com/tendermint/tendermint => github.com/oasisprotocol/tendermint v0.34.0-rc3-oasis1
	golang.org/x/crypto/curve25519 => github.com/oasisprotocol/ed25519/extra/x25519 v0.0.0-20200528083105-55566edd6df0
	golang.org/x/crypto/ed25519 => github.com/oasisprotocol/ed25519 v0.0.0-20200528083105-55566edd6df0
)

require (
	github.com/dgraph-io/badger/v2 v2.0.3 // indirect
	// https://github.com/oasisprotocol/oasis-core/releases/tag/v20.9
	github.com/oasisprotocol/oasis-core/go v0.20.9
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	gopkg.in/yaml.v2 v2.3.0
)
