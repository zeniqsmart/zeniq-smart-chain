package param

import (
	"math"
	"os"
	"path/filepath"

	"github.com/tendermint/tendermint/config"
)

const (
	DefaultRpcEthGetLogsMaxResults = 10000
	DefaultRetainBlocks            = -1
	DefaultNumKeptBlocks           = 10000
	DefaultNumKeptBlocksInDB       = -1
	DefaultSignatureCache          = 20000
	DefaultRecheckThreshold        = 1000
	DefaultTrunkCacheSize          = 200
	DefaultChangeRetainEveryN      = 100
	DefaultPruneEveryN             = 10

	DefaultAppDataPath    = "app"
	DefaultDbDataPath     = "db"
	DefaultSyncdbDataPath = "syncdb"
)

type AppConfig struct {
	//app config:
	AppDataPath    string `mapstructure:"app_data_path"`
	DbDataPath     string `mapstructure:"db_data_path"`
	SyncdbDataPath string `mapstructure:"syncdb_data_path"`
	// rpc config
	RpcEthGetLogsMaxResults int `mapstructure:"get_logs_max_results"`
	// tm db config
	RetainBlocks       int64 `mapstructure:"retain-blocks"`
	ChangeRetainEveryN int64 `mapstructure:"retain_interval_blocks"`
	// Use LiteDB instead of DB
	UseLiteDB bool `mapstructure:"use_litedb"`
	// the number of kept recent blocks for ads
	NumKeptBlocks int64 `mapstructure:"blocks_kept_ads"`
	// the number of kept recent blocks for db
	NumKeptBlocksInDB int64 `mapstructure:"blocks_kept_db"`
	// the entry count of the signature cache
	SigCacheSize   int   `mapstructure:"sig_cache_size"`
	TrunkCacheSize int   `mapstructure:"trunk_cache_size"`
	PruneEveryN    int64 `mapstructure:"prune_every_n"`
	// How many transactions are allowed to left in the mempool
	// If more than this threshold, no further transactions can go in mempool
	RecheckThreshold int `mapstructure:"recheck_threshold"`
	//watcher config
	MainnetRPCUrl      string `mapstructure:"mainnet-rpc-url"`
	MainnetRPCUsername string `mapstructure:"mainnet-rpc-username"`
	MainnetRPCPassword string `mapstructure:"mainnet-rpc-password"`
	ZeniqsmartRPCUrl   string `mapstructure:"zeniqsmart-rpc-url"`
	Speedup            bool   `mapstructure:"watcher-speedup"`
	Testing            bool   `mapstructure:"watcher-speedup"`

	FrontierGasLimit uint64 `mapstructure:"frontier-gaslimit"`

	ArchiveMode bool `mapstructure:"archive-mode"`

	WithSyncDB bool `mapstructure:"with-syncdb"`

	BlocksBehind   int64      `mapstructure:"blocks-behind"`
	UpdateOfADSLog bool       `mapstructure:"update-of-ads-log"`
	CCRPCEpochs    [][2]int64 `mapstructure:"cc-rpc-epochs"`
	CCRPCForkBlock int64      `mapstructure:"cc-rpc-fork-block"` // MaxInt64 to disable CCRPC
	HeightRevision [][2]int64 `mapstructure:"height-revision"`
}

type ChainConfig struct {
	NodeConfig *config.Config `mapstructure:"node_config"`
	AppConfig  *AppConfig     `mapstructure:"app_config"`
}

var (
	defaultHome = os.ExpandEnv("$HOME/.zeniqsmartd")
)

func DefaultAppConfig() *AppConfig {
	return DefaultAppConfigWithHome(defaultHome)
}
func DefaultAppConfigWithHome(home string) *AppConfig {
	if home == "" {
		home = defaultHome
	}
	return &AppConfig{
		AppDataPath:             filepath.Join(home, "data", DefaultAppDataPath),
		DbDataPath:              filepath.Join(home, "data", DefaultDbDataPath),
		SyncdbDataPath:          filepath.Join(home, "data", DefaultSyncdbDataPath),
		RpcEthGetLogsMaxResults: DefaultRpcEthGetLogsMaxResults,
		RetainBlocks:            DefaultRetainBlocks,
		NumKeptBlocks:           DefaultNumKeptBlocks,
		NumKeptBlocksInDB:       DefaultNumKeptBlocksInDB,
		SigCacheSize:            DefaultSignatureCache,
		RecheckThreshold:        DefaultRecheckThreshold,
		TrunkCacheSize:          DefaultTrunkCacheSize,
		ChangeRetainEveryN:      DefaultChangeRetainEveryN,
		PruneEveryN:             DefaultPruneEveryN,
		MainnetRPCUrl:           "",
		MainnetRPCUsername:      "zeniq",
		MainnetRPCPassword:      "zeniq123",
		ZeniqsmartRPCUrl:        "",
		Speedup:                 true,
		Testing:                 false,
		FrontierGasLimit:        uint64(BlockMaxGas / 200), //5Million gas
		ArchiveMode:             false,
		WithSyncDB:              false,
		BlocksBehind:            0,
		UpdateOfADSLog:          false,
		CCRPCEpochs:             [][2]int64{{184464, 6}},
		CCRPCForkBlock:          math.MaxInt64 - 1000,              // -1000 to allow tests set StartHeight beyond
		HeightRevision:          [][2]int64{{0, 7}, {14444444, 7}}, // 7=EVMC_ISTANBUL  is default from 0, 11=EVMC_SHANGHAI, next block > 14444444
	}
}

func DefaultConfig() *ChainConfig {
	c := &ChainConfig{
		NodeConfig: config.DefaultConfig(),
		AppConfig:  DefaultAppConfig(),
	}
	c.NodeConfig.TxIndex.Indexer = "null"
	return c
}
