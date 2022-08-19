// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package evmimpl

import (
	"github.com/iotaledger/wasp/packages/evm/evmtypes"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/subrealm"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/evm"
)

const (
	keyGasRatio = "g"

	// keyEVMState is the subrealm prefix for the EVM state, used by the emulator
	keyEVMState = "s"

	// keyISCMagic is the subrealm prefix for the ISCmagic contract
	keyISCMagic = "m"
)

func evmStateSubrealm(state kv.KVStore) kv.KVStore {
	return subrealm.New(state, keyEVMState)
}

func iscMagicSubrealm(state kv.KVStore) kv.KVStore {
	return subrealm.New(state, keyISCMagic)
}

func iscMagicSubrealmR(state kv.KVStoreReader) kv.KVStoreReader {
	return subrealm.NewReadOnly(state, keyISCMagic)
}

func setGasRatio(ctx isc.Sandbox) dict.Dict {
	ctx.RequireCallerIsChainOwner()
	ctx.State().Set(keyGasRatio, codec.MustDecodeRatio32(ctx.Params().MustGet(evm.FieldGasRatio)).Bytes())
	return nil
}

func getGasRatio(ctx isc.SandboxView) dict.Dict {
	return result(GetGasRatio(ctx.StateR()).Bytes())
}

func GetGasRatio(state kv.KVStoreReader) util.Ratio32 {
	return codec.MustDecodeRatio32(state.MustGet(keyGasRatio), evmtypes.DefaultGasRatio)
}
