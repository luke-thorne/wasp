// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package sandbox

import (
	"math/big"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/assert"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/execution"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

type SandboxBase struct {
	Ctx       execution.WaspContext
	assertObj *assert.Assert
}

var _ iscp.SandboxBase = &SandboxBase{}

func (s *SandboxBase) assert() *assert.Assert {
	if s.assertObj == nil {
		s.assertObj = assert.NewAssert(s.Ctx)
	}
	return s.assertObj
}

func (s *SandboxBase) AccountID() iscp.AgentID {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.AccountID()
}

func (s *SandboxBase) BalanceBaseTokens() uint64 {
	s.Ctx.GasBurn(gas.BurnCodeGetBalance)
	return s.Ctx.GetBaseTokensBalance(s.AccountID())
}

func (s *SandboxBase) BalanceNativeToken(id *iotago.NativeTokenID) *big.Int {
	s.Ctx.GasBurn(gas.BurnCodeGetBalance)
	return s.Ctx.GetNativeTokenBalance(s.AccountID(), id)
}

func (s *SandboxBase) BalanceFungibleTokens() *iscp.FungibleTokens {
	s.Ctx.GasBurn(gas.BurnCodeGetBalance)
	return s.Ctx.GetAssets(s.AccountID())
}

func (s *SandboxBase) OwnedNFTs() []iotago.NFTID {
	s.Ctx.GasBurn(gas.BurnCodeGetBalance)
	return s.Ctx.GetAccountNFTs(s.AccountID())
}

func (s *SandboxBase) GetNFTData(nftID iotago.NFTID) iscp.NFT {
	s.Ctx.GasBurn(gas.BurnCodeGetNFTData)
	return s.Ctx.GetNFTData(nftID)
}

func (s *SandboxBase) ChainID() *iscp.ChainID {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.ChainID()
}

func (s *SandboxBase) ChainOwnerID() iscp.AgentID {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.ChainOwnerID()
}

func (s *SandboxBase) Contract() iscp.Hname {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.CurrentContractHname()
}

func (s *SandboxBase) Timestamp() time.Time {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.Timestamp()
}

func (s *SandboxBase) Log() iscp.LogInterface {
	// TODO should Log be disabled for wasm contracts? not much of a point in exposing internal logging
	return s.Ctx
}

func (s *SandboxBase) Params() *iscp.Params {
	s.Ctx.GasBurn(gas.BurnCodeGetContext)
	return s.Ctx.Params()
}

func (s *SandboxBase) Utils() iscp.Utils {
	return NewUtils(s.Gas())
}

func (s *SandboxBase) Gas() iscp.Gas {
	return s
}

func (s *SandboxBase) Burn(burnCode gas.BurnCode, par ...uint64) {
	s.Ctx.GasBurn(burnCode, par...)
}

func (s *SandboxBase) Budget() uint64 {
	return s.Ctx.GasBudgetLeft()
}

// -- helper methods
func (s *SandboxBase) Requiref(cond bool, format string, args ...interface{}) {
	s.assert().Requiref(cond, format, args...)
}

func (s *SandboxBase) RequireNoError(err error, str ...string) {
	s.assert().RequireNoError(err, str...)
}

func (s *SandboxBase) CallView(contractHname, entryPoint iscp.Hname, params dict.Dict) dict.Dict {
	s.Ctx.GasBurn(gas.BurnCodeCallContract)
	if params == nil {
		params = make(dict.Dict)
	}
	return s.Ctx.Call(contractHname, entryPoint, params, nil)
}
