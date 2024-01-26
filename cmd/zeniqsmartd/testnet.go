package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ethereum/go-ethereum/common"

	"github.com/zeniqsmart/zeniq-smart-chain/app"
	"github.com/zeniqsmart/zeniq-smart-chain/param"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"

	"github.com/ethereum/go-ethereum/core"
)

var (
	nValidators    int
	nNonValidators int
	initialHeight  int64
	configFile     string
	outputDir      string
	nodeDirPrefix  string

	populatePersistentPeers bool
	hostnamePrefix          string
	hostnameSuffix          string
	startingIPAddress       string
	hostnames               []string
	p2pPort                 int
	randomMonikers          bool

	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))
)

const (
	nodeDirPerm = 0755
)

// TestnetFilesCmd allows initialisation of files for a Tendermint testnet.
var TestnetFilesCmd = &cobra.Command{
	Use:   "testnet",
	Short: "Initialize files for a zeniqsmartd testnet",
	Long: `testnet will create "v" + "n" number of directories and populate each with
necessary files (private validator, genesis, config, etc.).

Note, strict routability for addresses is turned off in the config file.

Optionally, it will fill in persistent_peers list in config file using either hostnames or IPs.

Example:

	./zeniqsmartd testnet --v 4 --o ./testnodes --populate-persistent-peers --starting-ip-address 192.168.10.2
	`,
	RunE: testnetFiles,
}

func AddTestnetCmd() *cobra.Command {
	TestnetFilesCmd.Flags().IntVar(&nValidators, "v", 4,
		"number of validators to initialize the testnet with")
	TestnetFilesCmd.Flags().StringVar(&configFile, "config", "",
		"config file to use (note some options may be overwritten)")
	TestnetFilesCmd.Flags().IntVar(&nNonValidators, "n", 0,
		"number of non-validators to initialize the testnet with")
	TestnetFilesCmd.Flags().StringVar(&outputDir, "o", "./mytestnet",
		"directory to store initialization data for the testnet")
	TestnetFilesCmd.Flags().StringVar(&nodeDirPrefix, "node-dir-prefix", "node",
		"prefix the directory name for each node with (node results in node0, node1, ...)")
	TestnetFilesCmd.Flags().Int64Var(&initialHeight, "initial-height", 0,
		"initial height of the first block")

	TestnetFilesCmd.Flags().BoolVar(&populatePersistentPeers, "populate-persistent-peers", true,
		"update config of each node with the list of persistent peers build using either"+
			" hostname-prefix or"+
			" starting-ip-address")
	TestnetFilesCmd.Flags().StringVar(&hostnamePrefix, "hostname-prefix", "node",
		"hostname prefix (\"node\" results in persistent peers list ID0@node0:26656, ID1@node1:26656, ...)")
	TestnetFilesCmd.Flags().StringVar(&hostnameSuffix, "hostname-suffix", "",
		"hostname suffix ("+
			"\".xyz.com\""+
			" results in persistent peers list ID0@node0.xyz.com:26656, ID1@node1.xyz.com:26656, ...)")
	TestnetFilesCmd.Flags().StringVar(&startingIPAddress, "starting-ip-address", "",
		"starting IP address ("+
			"\"192.168.0.1\""+
			" results in persistent peers list ID0@192.168.0.1:26656, ID1@192.168.0.2:26656, ...)")
	TestnetFilesCmd.Flags().StringArrayVar(&hostnames, "hostname", []string{},
		"manually override all hostnames of validators and non-validators (use --hostname multiple times for multiple hosts)")
	TestnetFilesCmd.Flags().IntVar(&p2pPort, "p2p-port", 26656,
		"P2P Port")
	TestnetFilesCmd.Flags().BoolVar(&randomMonikers, "random-monikers", false,
		"randomize the moniker for each generated node")
	return TestnetFilesCmd
}

func fromGenesisValidator(v *types.GenesisValidator) *app.Validator {
	return &app.Validator{
		Address:      common.HexToAddress(v.Address.String()),
		Pubkey:       common.HexToHash(hex.EncodeToString(v.PubKey.Bytes())),
		RewardTo:     common.HexToAddress(v.Address.String()),
		VotingPower:  1,
		Introduction: v.Name,
		IsRetiring:   false,
		MinerAddress: v.Address.Bytes(),
	}
}

func testnetFiles(cmd *cobra.Command, args []string) error {
	if len(hostnames) > 0 && len(hostnames) != (nValidators+nNonValidators) {
		return fmt.Errorf(
			"testnet needs precisely %d hostnames (number of validators plus non-validators) if --hostname parameter is used",
			nValidators+nNonValidators,
		)
	}

	config := cfg.DefaultConfig()

	// overwrite default config if set and valid
	if configFile != "" {
		viper.SetConfigFile(configFile)
		if err := viper.ReadInConfig(); err != nil {
			return err
		}
		if err := viper.Unmarshal(config); err != nil {
			return err
		}
		if err := config.ValidateBasic(); err != nil {
			return err
		}
	}

	genVals := make([]types.GenesisValidator, nValidators)
	vals := make([]*app.Validator, nValidators)

	for i := 0; i < nValidators; i++ {
		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName)
		config.SetRoot(nodeDir)

		err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
		err = os.MkdirAll(filepath.Join(nodeDir, "data"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		if err := initFilesWithConfig(config); err != nil {
			return err
		}

		pvKeyFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorKey)
		pvStateFile := filepath.Join(nodeDir, config.BaseConfig.PrivValidatorState)
		pv := privval.LoadFilePV(pvKeyFile, pvStateFile)

		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
		genVals[i] = types.GenesisValidator{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   1,
			Name:    nodeDirName,
		}
		vals[i] = fromGenesisValidator(&genVals[i])
	}

	for i := 0; i < nNonValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i+nValidators))
		config.SetRoot(nodeDir)

		err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		err = os.MkdirAll(filepath.Join(nodeDir, "data"), nodeDirPerm)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}

		if err := initFilesWithConfig(config); err != nil {
			return err
		}
	}

	// Generate genesis doc from generated validators
	genDoc := &types.GenesisDoc{
		ChainID:         randomChainID(),
		ConsensusParams: types.DefaultConsensusParams(),
		GenesisTime:     tmtime.Now(),
		InitialHeight:   initialHeight,
		Validators:      genVals,
	}

	accounts2make := make(map[common.Address]core.GenesisAccount)
	f2124, _, _ := big.ParseFloat("21e+24", 10, 0, big.ToNearestEven)
	var i2124 = new(big.Int)
	i2124, _ = f2124.Int(i2124)
	accounts2make[common.HexToAddress("0xf96ae03f3637e3195ebcb85f0043052338196e57")] =
		core.GenesisAccount{
			Balance: i2124,
		}

	genesisData := &app.GenesisData{
		Validators: vals,
		Alloc:      accounts2make,
	}

	genDoc.AppState, _ = json.Marshal(&genesisData)

	// Write genesis file.
	for i := 0; i < nValidators+nNonValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		if err := genDoc.SaveAs(filepath.Join(nodeDir, config.BaseConfig.Genesis)); err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Gather persistent peer addresses.
	var (
		persistentPeers string
		err             error
	)
	if populatePersistentPeers {
		persistentPeers, err = persistentPeersString(config)
		if err != nil {
			_ = os.RemoveAll(outputDir)
			return err
		}
	}

	// Overwrite default config.
	for i := 0; i < nValidators+nNonValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		config.P2P.AddrBookStrict = false
		config.P2P.AllowDuplicateIP = true
		if populatePersistentPeers {
			config.P2P.PersistentPeers = persistentPeers
		}
		config.Moniker = moniker(i)

		cfg.WriteConfigFile(filepath.Join(nodeDir, "config", "config.toml"), config)
	}

	// Overwrite app.toml
	for i := 0; i < nValidators+nNonValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		appConf := param.DefaultAppConfigWithHome("")
		appConfigFilePath := filepath.Join(nodeDir, "config", "app.toml")
		param.WriteConfigFile(appConfigFilePath, appConf)
	}

	fmt.Printf("Successfully initialized %v node directories\n", nValidators+nNonValidators)
	return nil
}

func hostnameOrIP(i int) string {
	if len(hostnames) > 0 && i < len(hostnames) {
		return hostnames[i]
	}
	if startingIPAddress == "" {
		return fmt.Sprintf("%s%d%s", hostnamePrefix, i, hostnameSuffix)
	}
	ip := net.ParseIP(startingIPAddress)
	ip = ip.To4()
	if ip == nil {
		fmt.Printf("%v: non ipv4 address\n", startingIPAddress)
		os.Exit(1)
	}

	for j := 0; j < i; j++ {
		ip[3]++
	}
	return ip.String()
}

func persistentPeersString(config *cfg.Config) (string, error) {
	persistentPeers := make([]string, nValidators+nNonValidators)
	for i := 0; i < nValidators+nNonValidators; i++ {
		nodeDir := filepath.Join(outputDir, fmt.Sprintf("%s%d", nodeDirPrefix, i))
		config.SetRoot(nodeDir)
		nodeKey, err := p2p.LoadNodeKey(config.NodeKeyFile())
		if err != nil {
			return "", err
		}
		persistentPeers[i] = p2p.IDAddressString(nodeKey.ID(), fmt.Sprintf("%s:%d", hostnameOrIP(i), p2pPort))
	}
	return strings.Join(persistentPeers, ","), nil
}

func moniker(i int) string {
	if randomMonikers {
		return randomMoniker()
	}
	if len(hostnames) > 0 && i < len(hostnames) {
		return hostnames[i]
	}
	if startingIPAddress == "" {
		return fmt.Sprintf("%s%d%s", hostnamePrefix, i, hostnameSuffix)
	}
	return randomMoniker()
}

func randomMoniker() string {
	return bytes.HexBytes(tmrand.Bytes(8)).String()
}

func randomChainID() string {
	return "0x" + bytes.HexBytes(tmrand.Bytes(6)).String()
}

func initFilesWithConfig(config *cfg.Config) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	var pv *privval.FilePV
	if tmos.FileExists(privValKeyFile) {
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv = privval.GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         randomChainID(),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
		}
		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
		genDoc.Validators = []types.GenesisValidator{{
			Address: pubKey.Address(),
			PubKey:  pubKey,
			Power:   10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}
