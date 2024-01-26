package staking_test

import (
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/zeniqsmart/zeniq-smart-chain/internal/ethutils"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/testutils"
	"github.com/zeniqsmart/zeniq-smart-chain/staking"
	"github.com/zeniqsmart/zeniq-smart-chain/staking/types"
)

// testdata/sol/contracts/staking/XHedgeStorage.sol
var xhedgeStorageCreationBytecode = testutils.HexToBytes(`
0x608060405234801561001057600080fd5b5061023e806100206000396000f3
fe608060405234801561001057600080fd5b50600436106100415760003560e0
1c806335aa2e4414610046578063430ce4f5146100765780639176355f146100
a6575b600080fd5b610060600480360381019061005b9190610158565b6100c2
565b60405161006d91906101cc565b60405180910390f35b6100906004803603
81019061008b9190610158565b6100e6565b60405161009d91906101cc565b60
405180910390f35b6100c060048036038101906100bb9190610181565b6100fe
565b005b608781815481106100d257600080fd5b906000526020600020016000
915090505481565b60866020528060005260406000206000915090505481565b
6087829080600181540180825580915050600190039060005260206000200160
0090919091909150558060866000848152602001908152602001600020819055
505050565b600081359050610152816101f1565b92915050565b600060208284
03121561016a57600080fd5b600061017884828501610143565b915050929150
50565b6000806040838503121561019457600080fd5b60006101a28582860161
0143565b92505060206101b385828601610143565b9150509250929050565b61
01c6816101e7565b82525050565b60006020820190506101e160008301846101
bd565b92915050565b6000819050919050565b6101fa816101e7565b81146102
0557600080fd5b5056fea26469706673582212207f4b721286f3f26fda9f7096
e41722438c2299799f97ee42a551ad5f9ef382b564736f6c63430008000033
`)

var xhedgeStorageABI = ethutils.MustParseABI(`
[
    {
      "inputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "name": "valToVotes",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "name": "validators",
      "outputs": [
        {
          "internalType": "uint256",
          "name": "",
          "type": "uint256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    },
    {
      "inputs": [
        {
          "internalType": "uint256",
          "name": "val",
          "type": "uint256"
        },
        {
          "internalType": "uint256",
          "name": "votes",
          "type": "uint256"
        }
      ],
      "name": "addVal",
      "outputs": [],
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ]
`)

func TestCreateInitVotes(t *testing.T) {
	key, addr := testutils.GenKeyAndAddr()
	_app := testutils.CreateTestApp(key)
	defer _app.Destroy()

	tx, _, xhedgeAddr := _app.DeployContractInBlock(key, xhedgeStorageCreationBytecode)
	_app.EnsureTxSuccess(tx.Hash())
	xhedgeSeq := _app.GetSeq(xhedgeAddr)
	require.True(t, xhedgeSeq > 0)

	ctx := _app.GetRunTxContext()
	staking.CreateInitVotes(ctx, xhedgeSeq, []*types.Validator{
		{Pubkey: uint256.NewInt(0x1234).Bytes32()},
		{Pubkey: uint256.NewInt(0x5678).Bytes32()},
		{Pubkey: uint256.NewInt(0xABCD).Bytes32()},
	})
	ctx.Close(true)
	_app.ExecTxsInBlock()

	require.Len(t, _app.GetDynamicArray(xhedgeAddr, staking.SlotValidatorsArray), 3)
	result := _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "validators", big.NewInt(0))
	require.Equal(t, []interface{}{big.NewInt(0x1234)}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "validators", big.NewInt(1))
	require.Equal(t, []interface{}{big.NewInt(0x5678)}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "validators", big.NewInt(2))
	require.Equal(t, []interface{}{big.NewInt(0xABCD)}, result)

	one := big.NewInt(1)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", big.NewInt(0x1234))
	require.Equal(t, []interface{}{one}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", big.NewInt(0x5678))
	require.Equal(t, []interface{}{one}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", big.NewInt(0xABCD))
	require.Equal(t, []interface{}{one}, result)
}

func TestGetAndClearPosVotes(t *testing.T) {
	key, addr := testutils.GenKeyAndAddr()
	_app := testutils.CreateTestApp(key)
	defer _app.Destroy()

	tx, _, xhedgeAddr := _app.DeployContractInBlock(key, xhedgeStorageCreationBytecode)
	_app.EnsureTxSuccess(tx.Hash())
	xhedgeSeq := _app.GetSeq(xhedgeAddr)
	require.True(t, xhedgeSeq > 0)

	val1Key := big.NewInt(0x1234)
	val2Key := big.NewInt(0xABCD)
	val1Votes := uint256.NewInt(0).Mul(uint256.NewInt(2), staking.CoindayUnit).ToBig()
	val2Votes := uint256.NewInt(0).Mul(uint256.NewInt(3), staking.CoindayUnit).ToBig()

	data := xhedgeStorageABI.MustPack("addVal", val1Key, val1Votes)
	tx, _ = _app.MakeAndExecTxInBlock(key, xhedgeAddr, 0, data)
	_app.EnsureTxSuccess(tx.Hash())

	data = xhedgeStorageABI.MustPack("addVal", val2Key, val2Votes)
	tx, _ = _app.MakeAndExecTxInBlock(key, xhedgeAddr, 0, data)
	_app.EnsureTxSuccess(tx.Hash())

	result := _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "validators", big.NewInt(0))
	require.Equal(t, []interface{}{val1Key}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "validators", big.NewInt(1))
	require.Equal(t, []interface{}{val2Key}, result)

	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", val1Key)
	require.Equal(t, []interface{}{val1Votes}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", val2Key)
	require.Equal(t, []interface{}{val2Votes}, result)

	ctx := _app.GetRunTxContext()
	posVotes := staking.GetAndClearPosVotes(ctx, xhedgeSeq)
	ctx.Close(true)
	require.Len(t, posVotes, 2)
	require.Equal(t, map[[32]byte]int64{
		uint256.NewInt(0x1234).Bytes32(): 2,
		uint256.NewInt(0xABCD).Bytes32(): 3,
	}, posVotes)

	_app.ExecTxsInBlock()
	require.Len(t, _app.GetDynamicArray(xhedgeAddr, staking.SlotValidatorsArray), 0)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", val1Key)
	require.Equal(t, []interface{}{big.NewInt(1).SetInt64(0)}, result)
	result = _app.CallWithABI(addr, xhedgeAddr, xhedgeStorageABI, "valToVotes", val2Key)
	require.Equal(t, []interface{}{big.NewInt(1).SetInt64(0)}, result)

	ctx = _app.GetRunTxContext()
	posVotes = staking.GetAndClearPosVotes(ctx, xhedgeSeq)
	ctx.Close(true)
	require.Len(t, posVotes, 0)
}
