package solo

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotaledger/wasp/packages/evm/evmtypes"
	"github.com/iotaledger/wasp/packages/evm/jsonrpc"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
	"github.com/stretchr/testify/require"
)

type jsonRPCSoloBackend struct {
	Chain     *Chain
	baseToken *parameters.BaseToken
}

func newJSONRPCSoloBackend(chain *Chain, baseToken *parameters.BaseToken) jsonrpc.ChainBackend {
	return &jsonRPCSoloBackend{Chain: chain, baseToken: baseToken}
}

func (b *jsonRPCSoloBackend) EVMSendTransaction(tx *types.Transaction) error {
	_, err := b.Chain.PostEthereumTransaction(tx)
	return err
}

func (b *jsonRPCSoloBackend) EVMEstimateGas(callMsg ethereum.CallMsg) (uint64, error) {
	return b.Chain.EstimateGasEthereum(callMsg)
}

func (b *jsonRPCSoloBackend) ISCCallView(scName, funName string, args dict.Dict) (dict.Dict, error) {
	return b.Chain.CallView(scName, funName, args)
}

func (b *jsonRPCSoloBackend) BaseToken() *parameters.BaseToken {
	return b.baseToken
}

func (ch *Chain) EVM() *jsonrpc.EVMChain {
	ret, err := ch.CallView(evm.Contract.Name, evm.FuncGetChainID.Name)
	require.NoError(ch.Env.T, err)
	return jsonrpc.NewEVMChain(
		newJSONRPCSoloBackend(ch, parameters.L1.BaseToken),
		evmtypes.MustDecodeChainID(ret.MustGet(evm.FieldResult)),
	)
}

func (ch *Chain) EVMGasRatio() util.Ratio32 {
	// TODO: Cache the gas ratio?
	ret, err := ch.CallView(evm.Contract.Name, evm.FuncGetGasRatio.Name)
	require.NoError(ch.Env.T, err)
	return codec.MustDecodeRatio32(ret.MustGet(evm.FieldResult))
}

func (ch *Chain) PostEthereumTransaction(tx *types.Transaction) (dict.Dict, error) {
	req, err := isc.NewEVMOffLedgerRequest(ch.ChainID, tx)
	if err != nil {
		return nil, err
	}
	return ch.RunOffLedgerRequest(req)
}

func (ch *Chain) EstimateGasEthereum(callMsg ethereum.CallMsg) (uint64, error) {
	res := ch.estimateGas(isc.NewEVMOffLedgerEstimateGasRequest(ch.ChainID, callMsg))
	if res.Receipt.Error != nil {
		return 0, res.Receipt.Error
	}
	return codec.DecodeUint64(res.Return.MustGet(evm.FieldResult))
}

func NewEthereumAccount() (*ecdsa.PrivateKey, common.Address) {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	return key, crypto.PubkeyToAddress(key.PublicKey)
}

func (ch *Chain) NewEthereumAccountWithL2Funds(baseTokens ...uint64) (*ecdsa.PrivateKey, common.Address) {
	key, addr := NewEthereumAccount()
	ch.GetL2FundsFromFaucet(isc.NewEthereumAddressAgentID(addr), baseTokens...)
	return key, addr
}
