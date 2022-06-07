package solo

import (
	"fmt"
	"math"
	"math/big"
	"sort"

	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/stretchr/testify/require"
)

// L2Accounts returns all accounts on the chain with non-zero balances
func (ch *Chain) L2Accounts() []iscp.AgentID {
	d, err := ch.CallView(accounts.Contract.Name, accounts.ViewAccounts.Name)
	require.NoError(ch.Env.T, err)
	keys := d.KeysSorted()
	ret := make([]iscp.AgentID, 0, len(keys)-1)
	for _, key := range keys {
		aid, err := codec.DecodeAgentID([]byte(key))
		require.NoError(ch.Env.T, err)
		ret = append(ret, aid)
	}
	return ret
}

func (ch *Chain) parseAccountBalance(d dict.Dict, err error) *iscp.FungibleTokens {
	require.NoError(ch.Env.T, err)
	if d.IsEmpty() {
		return iscp.NewEmptyAssets()
	}
	ret, err := iscp.FungibleTokensFromDict(d)
	require.NoError(ch.Env.T, err)
	return ret
}

func (ch *Chain) L2Ledger() map[string]*iscp.FungibleTokens {
	accs := ch.L2Accounts()
	ret := make(map[string]*iscp.FungibleTokens)
	for i := range accs {
		ret[string(accs[i].Bytes())] = ch.L2Assets(accs[i])
	}
	return ret
}

func (ch *Chain) L2LedgerString() string {
	l := ch.L2Ledger()
	keys := make([]string, 0, len(l))
	for aid := range l {
		keys = append(keys, aid)
	}
	sort.Strings(keys)
	ret := ""
	for _, aid := range keys {
		ret += aid + "\n"
		ret += "        " + l[aid].String() + "\n"
	}
	return ret
}

// L2Assets return all tokens contained in the on-chain account controlled by the 'agentID'
func (ch *Chain) L2Assets(agentID iscp.AgentID) *iscp.FungibleTokens {
	return ch.parseAccountBalance(
		ch.CallView(accounts.Contract.Name, accounts.ViewBalance.Name, accounts.ParamAgentID, agentID),
	)
}

func (ch *Chain) L2Iotas(agentID iscp.AgentID) uint64 {
	return ch.L2Assets(agentID).Iotas
}

func (ch *Chain) L2NFTs(agentID iscp.AgentID) []iotago.NFTID {
	ret := make([]iotago.NFTID, 0)
	res, err := ch.CallView(accounts.Contract.Name, accounts.ViewAccountNFTs.Name, accounts.ParamAgentID, agentID)
	require.NoError(ch.Env.T, err)
	nftIDs := collections.NewArray16ReadOnly(res, accounts.ParamNFTIDs)
	nftLen := nftIDs.MustLen()
	for i := uint16(0); i < nftLen; i++ {
		nftID := iotago.NFTID{}
		copy(nftID[:], nftIDs.MustGetAt(i))
		ret = append(ret, nftID)
	}
	return ret
}

func (ch *Chain) L2NativeTokens(agentID iscp.AgentID, tokenID *iotago.NativeTokenID) *big.Int {
	return ch.L2Assets(agentID).AmountNativeToken(tokenID)
}

func (ch *Chain) L2CommonAccountAssets() *iscp.FungibleTokens {
	return ch.L2Assets(ch.CommonAccount())
}

func (ch *Chain) L2CommonAccountIotas() uint64 {
	return ch.L2Assets(ch.CommonAccount()).Iotas
}

func (ch *Chain) L2CommonAccountNativeTokens(tokenID *iotago.NativeTokenID) *big.Int {
	return ch.L2Assets(ch.CommonAccount()).AmountNativeToken(tokenID)
}

// L2TotalAssets return total sum of ftokens contained in the on-chain accounts
func (ch *Chain) L2TotalAssets() *iscp.FungibleTokens {
	return ch.parseAccountBalance(
		ch.CallView(accounts.Contract.Name, accounts.ViewTotalAssets.Name),
	)
}

// L2TotalIotas return total sum of iotas in L2 (all accounts)
func (ch *Chain) L2TotalIotas() uint64 {
	return ch.L2TotalAssets().Iotas
}

func mustNativeTokenIDFromBytes(data []byte) *iotago.NativeTokenID {
	if len(data) != iotago.NativeTokenIDLength {
		panic("len(data) != iotago.NativeTokenIDLength")
	}
	ret := new(iotago.NativeTokenID)
	copy(ret[:], data)
	return ret
}

func (ch *Chain) GetOnChainTokenIDs() []*iotago.NativeTokenID {
	res, err := ch.CallView(accounts.Contract.Name, accounts.ViewGetNativeTokenIDRegistry.Name)
	require.NoError(ch.Env.T, err)
	ret := make([]*iotago.NativeTokenID, 0, len(res))
	for k := range res {
		ret = append(ret, mustNativeTokenIDFromBytes([]byte(k)))
	}
	return ret
}

func (ch *Chain) GetFoundryOutput(sn uint32) (*iotago.FoundryOutput, error) {
	res, err := ch.CallView(accounts.Contract.Name, accounts.ViewFoundryOutput.Name,
		accounts.ParamFoundrySN, sn,
	)
	if err != nil {
		return nil, err
	}
	outBin := res.MustGet(accounts.ParamFoundryOutputBin)
	out := &iotago.FoundryOutput{}
	_, err = out.Deserialize(outBin, serializer.DeSeriModeNoValidation, nil)
	require.NoError(ch.Env.T, err)
	return out, nil
}

func (ch *Chain) GetNativeTokenIDByFoundrySN(sn uint32) (iotago.NativeTokenID, error) {
	o, err := ch.GetFoundryOutput(sn)
	if err != nil {
		return iotago.NativeTokenID{}, err
	}
	return o.MustNativeTokenID(), nil
}

type foundryParams struct {
	ch   *Chain
	user *cryptolib.KeyPair
	sch  iotago.TokenScheme
}

// CreateFoundryGasBudgetIotas always takes 100000 iotas as gas budget and ftokens for the call
const (
	DestroyTokensGasBudgetIotas       = 1 * iscp.Mi
	SendToL2AccountGasBudgetIotas     = 1 * iscp.Mi
	DestroyFoundryGasBudgetIotas      = 1 * iscp.Mi
	TransferAllowanceToGasBudgetIotas = 1 * iscp.Mi
)

func (ch *Chain) NewFoundryParams(maxSupply interface{}) *foundryParams {
	ret := &foundryParams{
		ch: ch,
		sch: &iotago.SimpleTokenScheme{
			MaximumSupply: util.ToBigInt(maxSupply),
			MeltedTokens:  big.NewInt(0),
			MintedTokens:  big.NewInt(0),
		},
	}
	return ret
}

func (fp *foundryParams) WithUser(user *cryptolib.KeyPair) *foundryParams {
	fp.user = user
	return fp
}

func (fp *foundryParams) WithTokenScheme(sch iotago.TokenScheme) *foundryParams {
	fp.sch = sch
	return fp
}

const (
	allowanceForFoundryDustDeposit = 1 * iscp.Mi
	allowanceForModifySupply       = 1 * iscp.Mi
)

func (fp *foundryParams) CreateFoundry() (uint32, iotago.NativeTokenID, error) {
	par := dict.New()
	if fp.sch != nil {
		par.Set(accounts.ParamTokenScheme, codec.EncodeTokenScheme(fp.sch))
	}
	user := fp.ch.OriginatorPrivateKey
	if fp.user != nil {
		user = fp.user
	}
	req := NewCallParamsFromDict(accounts.Contract.Name, accounts.FuncFoundryCreateNew.Name, par).
		WithAllowance(iscp.NewAllowanceIotas(allowanceForFoundryDustDeposit))

	gas, _, err := fp.ch.EstimateGasOnLedger(req, user, true)
	if err != nil {
		return 0, iotago.NativeTokenID{}, err
	}
	req.WithGasBudget(gas)
	res, err := fp.ch.PostRequestSync(req, user)
	if err != nil {
		return 0, iotago.NativeTokenID{}, err
	}
	resDeco := kvdecoder.New(res)
	retSN := resDeco.MustGetUint32(accounts.ParamFoundrySN)
	tokenID, err := fp.ch.GetNativeTokenIDByFoundrySN(retSN)
	return retSN, tokenID, err
}

func toFoundrySN(foundry interface{}) uint32 {
	switch f := foundry.(type) {
	case uint32:
		return f
	case *iotago.NativeTokenID:
		return f.FoundrySerialNumber()
	case iotago.NativeTokenID:
		return f.FoundrySerialNumber()
	}
	panic(fmt.Sprintf("toFoundrySN: type %T not supported", foundry))
}

func (ch *Chain) DestroyFoundry(sn uint32, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.Contract.Name, accounts.FuncFoundryDestroy.Name,
		accounts.ParamFoundrySN, sn).
		WithGasBudget(DestroyFoundryGasBudgetIotas)
	_, err := ch.PostRequestSync(req, user)
	return err
}

func (ch *Chain) MintTokens(foundry, amount interface{}, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.Contract.Name, accounts.FuncFoundryModifySupply.Name,
		accounts.ParamFoundrySN, toFoundrySN(foundry),
		accounts.ParamSupplyDeltaAbs, util.ToBigInt(amount),
	).
		WithAllowance(iscp.NewAllowance(allowanceForModifySupply, nil, nil)) // enough allowance is needed for the dust deposit when token is minted first on the chain
	g, _, err := ch.EstimateGasOnLedger(req, user, true)
	if err != nil {
		return err
	}

	req.WithGasBudget(g)
	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err = ch.PostRequestSync(req, user)
	return err
}

// DestroyTokensOnL2 destroys tokens (identified by foundry SN) on user's on-chain account
func (ch *Chain) DestroyTokensOnL2(foundryOrTokenID, amount interface{}, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.Contract.Name, accounts.FuncFoundryModifySupply.Name,
		accounts.ParamFoundrySN, toFoundrySN(foundryOrTokenID),
		accounts.ParamSupplyDeltaAbs, util.ToBigInt(amount),
		accounts.ParamDestroyTokens, true,
	).WithGasBudget(DestroyTokensGasBudgetIotas)

	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err := ch.PostRequestSync(req, user)
	return err
}

// DestroyTokensOnL1 sends tokens as ftokens and destroys in the same transaction
func (ch *Chain) DestroyTokensOnL1(tokenID *iotago.NativeTokenID, amount interface{}, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.Contract.Name, accounts.FuncFoundryModifySupply.Name,
		accounts.ParamFoundrySN, toFoundrySN(tokenID),
		accounts.ParamSupplyDeltaAbs, util.ToBigInt(amount),
		accounts.ParamDestroyTokens, true,
	).WithGasBudget(DestroyTokensGasBudgetIotas).AddIotas(1000)
	req.AddNativeTokens(tokenID, amount)
	req.AddAllowanceNativeTokens(tokenID, amount)
	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err := ch.PostRequestSync(req, user)
	return err
}

// DepositAssetsToL2 deposits ftokens on user's on-chain account
func (ch *Chain) DepositAssetsToL2(assets *iscp.FungibleTokens, user *cryptolib.KeyPair) error {
	_, err := ch.PostRequestSync(
		NewCallParams(accounts.Contract.Name, accounts.FuncDeposit.Name).
			WithFungibleTokens(assets).
			WithGasBudget(math.MaxUint64),
		user,
	)
	return err
}

// TransferAllowanceTo sends an on-ledger request to transfer funds to target account (sends extra iotas to the sender account to cover gas)
func (ch *Chain) TransferAllowanceTo(allowance *iscp.FungibleTokens, targetAccount iscp.AgentID, forceOpenAccount bool, wallet *cryptolib.KeyPair) error {
	_, err := ch.PostRequestSync(
		NewCallParams(accounts.Contract.Name, accounts.FuncTransferAllowanceTo.Name, dict.Dict{
			accounts.ParamAgentID:          codec.EncodeAgentID(targetAccount),
			accounts.ParamForceOpenAccount: codec.EncodeBool(forceOpenAccount),
		}).
			WithAllowance(iscp.NewAllowanceFungibleTokens(allowance)).
			WithFungibleTokens(allowance.Clone().AddIotas(TransferAllowanceToGasBudgetIotas)).
			WithGasBudget(math.MaxUint64),
		wallet,
	)
	return err
}

// DepositIotasToL2 deposits ftokens on user's on-chain account
func (ch *Chain) DepositIotasToL2(amount uint64, user *cryptolib.KeyPair) error {
	return ch.DepositAssetsToL2(iscp.NewFungibleTokens(amount, nil), user)
}

func (ch *Chain) MustDepositIotasToL2(amount uint64, user *cryptolib.KeyPair) {
	err := ch.DepositIotasToL2(amount, user)
	require.NoError(ch.Env.T, err)
}

// SendFromL1ToL2Account sends ftokens from L1 address to the target account on L2
// Sender pays the gas fee
func (ch *Chain) SendFromL1ToL2Account(totalIotas uint64, toSend *iscp.FungibleTokens, target iscp.AgentID, user *cryptolib.KeyPair) error {
	require.False(ch.Env.T, toSend.IsEmpty())
	sumAssets := toSend.Clone().AddIotas(totalIotas)
	_, err := ch.PostRequestSync(
		NewCallParams(accounts.Contract.Name, accounts.FuncTransferAllowanceTo.Name, accounts.ParamAgentID, target).
			AddFungibleTokens(sumAssets).
			AddAllowance(iscp.NewAllowanceFungibleTokens(toSend)).
			WithGasBudget(math.MaxUint64),
		user,
	)
	return err
}

func (ch *Chain) SendFromL1ToL2AccountIotas(totalIotas, iotasSend uint64, target iscp.AgentID, user *cryptolib.KeyPair) error {
	return ch.SendFromL1ToL2Account(totalIotas, iscp.NewTokensIotas(iotasSend), target, user)
}

// SendFromL2ToL2Account moves ftokens on L2 from user's account to the target
func (ch *Chain) SendFromL2ToL2Account(transfer *iscp.Allowance, target iscp.AgentID, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.Contract.Name, accounts.FuncTransferAllowanceTo.Name,
		accounts.ParamAgentID, target)

	req.AddIotas(SendToL2AccountGasBudgetIotas).
		AddAllowance(transfer).
		WithGasBudget(SendToL2AccountGasBudgetIotas)
	_, err := ch.PostRequestSync(req, user)
	return err
}

func (ch *Chain) SendFromL2ToL2AccountIotas(iotas uint64, target iscp.AgentID, user *cryptolib.KeyPair) error {
	return ch.SendFromL2ToL2Account(iscp.NewAllowance(iotas, nil, nil), target, user)
}

func (ch *Chain) SendFromL2ToL2AccountNativeTokens(id iotago.NativeTokenID, target iscp.AgentID, amount interface{}, user *cryptolib.KeyPair) error {
	transfer := iscp.NewEmptyAllowance()
	transfer.Assets.AddNativeTokens(id, amount)
	return ch.SendFromL2ToL2Account(transfer, target, user)
}
