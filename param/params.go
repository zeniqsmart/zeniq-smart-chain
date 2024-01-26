//go:build !params_testnet && !params_amber
// +build !params_testnet,!params_amber

package param

// FILE: consensus configurable params collected here!
const (
	/**app consensus params**/
	BlockMaxBytes      int64  = 4 * 1024 * 1024 // 4MB
	BlockMaxGas        int64  = 1_000_000_000   // 1Billion
	DefaultMinGasPrice uint64 = 10_000_000_000  // 10gwei

	/**ebp consensus params**/
	EbpExeRoundCount int = 200
	EbpRunnerNumber  int = 256
	EbpParallelNum   int = 32

	// gas limit for each transaction
	MaxTxGasLimit uint64 = 1000_0000

	/**staking consensus params**/
	// reward params
	EpochCountBeforeRewardMature  int64  = 1
	ProposerBaseMintFeePercentage uint64 = 15
	CollectorMintFeePercentage    uint64 = 15

	// epoch params
	StakingMinVotingPercentPerEpoch        int   = 10 //10 percent in StakingNumBlocksInEpoch, like 2016 / 10 = 201
	StakingMinVotingPubKeysPercentPerEpoch int   = 34 //34 percent in active validators,
	StakingNumBlocksInEpoch                int64 = 2016
	StakingEpochSwitchDelay                int64 = 600 * 2016 / 20 // 5% time of an epoch
	MaxActiveValidatorCount                int   = 50

	// ccEpoch params
	BlocksInCCEpoch    int64 = 7
	CCEpochSwitchDelay int64 = 3 * 20 / 20

	// network params
	IsAmber                           bool  = false
	AmberBlocksInEpochAfterXHedgeFork int64 = 2016 * 10 * 60 / 6

	// fork params
	XHedgeContractSequence uint64 = 0x13311
	XHedgeForkBlock        int64  = 4106000
	ShaGateForkBlock       int64  = 80000000
	ShaGateSwitch          bool   = false

	CCRPCForkBlock int64 = 11000011
	// must be earlier than CCRPCForkBlock - CCRPCEpochs[.][0]
	CCRPCMAINNET int64 = 1008 * 183 // 184464, (184464 + 1008)/2016 == 92
	// CCRPCMAINNET = CCRPCEpochs[0][0]
)
