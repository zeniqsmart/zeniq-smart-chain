package param

import (
	"bytes"
	"text/template"

	"github.com/spf13/viper"
	tmos "github.com/tendermint/tendermint/libs/os"
)

const defaultConfigTemplate = `# This is a TOML config file.
# For more information, see https://github.com/toml-lang/toml

# eth_getLogs max return items
get_logs_max_results = {{ .RpcEthGetLogsMaxResults }}

# retain blocks in TM
retain-blocks = {{ .RetainBlocks }}

# every retain_interval_blocks blocks execute TM blocks prune
retain_interval_blocks = {{ .ChangeRetainEveryN }}

# use liteDB
use_litedb = {{ .UseLiteDB }}

# How many recent blocks can be kept in ads (to prune the blocks which are older than them)
blocks_kept_ads = {{ .NumKeptBlocks }}

# How many recent blocks can be kept in db (to prune the blocks which are older than them)
blocks_kept_db = {{ .NumKeptBlocksInDB }}

# The entry count limit of the signature cache, which caches the recent signatures' check results
sig_cache_size = {{ .SigCacheSize }}

# The initial entry count in the trunk cache, which buffers the write operations of the last block
trunk_cache_size = {{ .TrunkCacheSize }}

# We try to prune the old blocks of ads every n blocks
prune_every_n = {{ .PruneEveryN }}

# If the number of the mempool transactions which need recheck is larger than this threshold, stop
# adding new transactions into mempool
recheck_threshold = {{ .RecheckThreshold }}

# Zeniq mainnet rpc url
mainnet-rpc-url = "{{ .MainnetRPCUrl }}"

# Zeniq mainnet rpc username
mainnet-rpc-username = "{{ .MainnetRPCUsername }}"

# Zeniq mainnet rpc password
mainnet-rpc-password = "{{ .MainnetRPCPassword }}"

# zeniqsmart rpc url for epoch get
zeniqsmart-rpc-url = "{{ .ZeniqsmartRPCUrl }}"

# open epoch get to speedup mainnet block catch, work with "zeniqsmart_rpc_url"
watcher-speedup = {{ .Speedup }}

testing = {{ .Testing }}

frontier-gaslimit = {{ .FrontierGasLimit }}

archive-mode = {{ .ArchiveMode }}

with-syncdb = {{ .WithSyncDB }}

update-of-ads-log = {{ .UpdateOfADSLog }}
blocks-behind = {{ .BlocksBehind }}

# format: [[mainnetHeight>=184464,mainnetBlocks>=6],...]
# more entries but at least one
# no more than one entry here because if the actual one is less entries than here, default stays default
# keep this small default value for the tests
# cc-rpc-epochs = {{ .CCRPCEpochs }} # does not make commas
cc-rpc-epochs = [ [184464, 6] ]
cc-rpc-fork-block = {{ .CCRPCForkBlock }}

# height-revision = {{ .HeightRevision }} # does not make commas
height-revision = [ [0, 7], [17777777, 11] ]

`

var configTemplate *template.Template

func init() {
	var err error
	tmpl := template.New("appConfigFileTemplate")
	if configTemplate, err = tmpl.Parse(defaultConfigTemplate); err != nil {
		panic(err)
	}
}

func ParseConfig(home string) (*AppConfig, error) {
	conf := DefaultAppConfigWithHome(home)
	err := viper.Unmarshal(conf)
	return conf, err
}

func WriteConfigFile(configFilePath string, config *AppConfig) {
	var buffer bytes.Buffer
	if err := configTemplate.Execute(&buffer, config); err != nil {
		panic(err)
	}
	tmos.MustWriteFile(configFilePath, buffer.Bytes(), 0644)
}
