// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmtypes from "wasmlib/wasmtypes";
import * as sc from "./index";

export class ImmutableCallOnChainParams extends wasmtypes.ScProxy {
	hnameContract(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHnameContract));
	}

	hnameEP(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHnameEP));
	}

	n(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class MutableCallOnChainParams extends wasmtypes.ScProxy {
	hnameContract(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHnameContract));
	}

	hnameEP(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHnameEP));
	}

	n(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class ImmutableCheckContextFromFullEPParams extends wasmtypes.ScProxy {
	agentID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	caller(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamCaller));
	}

	chainID(): wasmtypes.ScImmutableChainID {
		return new wasmtypes.ScImmutableChainID(this.proxy.root(sc.ParamChainID));
	}

	chainOwnerID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamChainOwnerID));
	}
}

export class MutableCheckContextFromFullEPParams extends wasmtypes.ScProxy {
	agentID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	caller(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamCaller));
	}

	chainID(): wasmtypes.ScMutableChainID {
		return new wasmtypes.ScMutableChainID(this.proxy.root(sc.ParamChainID));
	}

	chainOwnerID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamChainOwnerID));
	}
}

export class ImmutableInitParams extends wasmtypes.ScProxy {
	fail(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamFail));
	}
}

export class MutableInitParams extends wasmtypes.ScProxy {
	fail(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamFail));
	}
}

export class ImmutablePassTypesFullParams extends wasmtypes.ScProxy {
	address(): wasmtypes.ScImmutableAddress {
		return new wasmtypes.ScImmutableAddress(this.proxy.root(sc.ParamAddress));
	}

	agentID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScImmutableChainID {
		return new wasmtypes.ScImmutableChainID(this.proxy.root(sc.ParamChainID));
	}

	contractID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamContractID));
	}

	hash(): wasmtypes.ScImmutableHash {
		return new wasmtypes.ScImmutableHash(this.proxy.root(sc.ParamHash));
	}

	hname(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHname));
	}

	hnameZero(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHnameZero));
	}

	int64(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamInt64));
	}

	int64Zero(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamInt64Zero));
	}

	string(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamString));
	}

	stringZero(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamStringZero));
	}
}

export class MutablePassTypesFullParams extends wasmtypes.ScProxy {
	address(): wasmtypes.ScMutableAddress {
		return new wasmtypes.ScMutableAddress(this.proxy.root(sc.ParamAddress));
	}

	agentID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScMutableChainID {
		return new wasmtypes.ScMutableChainID(this.proxy.root(sc.ParamChainID));
	}

	contractID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamContractID));
	}

	hash(): wasmtypes.ScMutableHash {
		return new wasmtypes.ScMutableHash(this.proxy.root(sc.ParamHash));
	}

	hname(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHname));
	}

	hnameZero(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHnameZero));
	}

	int64(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamInt64));
	}

	int64Zero(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamInt64Zero));
	}

	string(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamString));
	}

	stringZero(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamStringZero));
	}
}

export class ImmutableRunRecursionParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class MutableRunRecursionParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class ImmutableSetIntParams extends wasmtypes.ScProxy {
	intValue(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamIntValue));
	}

	name(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamName));
	}
}

export class MutableSetIntParams extends wasmtypes.ScProxy {
	intValue(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamIntValue));
	}

	name(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamName));
	}
}

export class ImmutableSpawnParams extends wasmtypes.ScProxy {
	progHash(): wasmtypes.ScImmutableHash {
		return new wasmtypes.ScImmutableHash(this.proxy.root(sc.ParamProgHash));
	}
}

export class MutableSpawnParams extends wasmtypes.ScProxy {
	progHash(): wasmtypes.ScMutableHash {
		return new wasmtypes.ScMutableHash(this.proxy.root(sc.ParamProgHash));
	}
}

export class ImmutableTestEventLogGenericDataParams extends wasmtypes.ScProxy {
	counter(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamCounter));
	}
}

export class MutableTestEventLogGenericDataParams extends wasmtypes.ScProxy {
	counter(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamCounter));
	}
}

export class ImmutableWithdrawFromChainParams extends wasmtypes.ScProxy {
	chainID(): wasmtypes.ScImmutableChainID {
		return new wasmtypes.ScImmutableChainID(this.proxy.root(sc.ParamChainID));
	}

	gasBudget(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamGasBudget));
	}

	iotasWithdrawal(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamIotasWithdrawal));
	}
}

export class MutableWithdrawFromChainParams extends wasmtypes.ScProxy {
	chainID(): wasmtypes.ScMutableChainID {
		return new wasmtypes.ScMutableChainID(this.proxy.root(sc.ParamChainID));
	}

	gasBudget(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamGasBudget));
	}

	iotasWithdrawal(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamIotasWithdrawal));
	}
}

export class ImmutableCheckContextFromViewEPParams extends wasmtypes.ScProxy {
	agentID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScImmutableChainID {
		return new wasmtypes.ScImmutableChainID(this.proxy.root(sc.ParamChainID));
	}

	chainOwnerID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamChainOwnerID));
	}
}

export class MutableCheckContextFromViewEPParams extends wasmtypes.ScProxy {
	agentID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScMutableChainID {
		return new wasmtypes.ScMutableChainID(this.proxy.root(sc.ParamChainID));
	}

	chainOwnerID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamChainOwnerID));
	}
}

export class ImmutableFibonacciParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class MutableFibonacciParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class ImmutableFibonacciIndirectParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScImmutableUint64 {
		return new wasmtypes.ScImmutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class MutableFibonacciIndirectParams extends wasmtypes.ScProxy {
	n(): wasmtypes.ScMutableUint64 {
		return new wasmtypes.ScMutableUint64(this.proxy.root(sc.ParamN));
	}
}

export class ImmutableGetIntParams extends wasmtypes.ScProxy {
	name(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamName));
	}
}

export class MutableGetIntParams extends wasmtypes.ScProxy {
	name(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamName));
	}
}

export class ImmutableGetStringValueParams extends wasmtypes.ScProxy {
	varName(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamVarName));
	}
}

export class MutableGetStringValueParams extends wasmtypes.ScProxy {
	varName(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamVarName));
	}
}

export class ImmutablePassTypesViewParams extends wasmtypes.ScProxy {
	address(): wasmtypes.ScImmutableAddress {
		return new wasmtypes.ScImmutableAddress(this.proxy.root(sc.ParamAddress));
	}

	agentID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScImmutableChainID {
		return new wasmtypes.ScImmutableChainID(this.proxy.root(sc.ParamChainID));
	}

	contractID(): wasmtypes.ScImmutableAgentID {
		return new wasmtypes.ScImmutableAgentID(this.proxy.root(sc.ParamContractID));
	}

	hash(): wasmtypes.ScImmutableHash {
		return new wasmtypes.ScImmutableHash(this.proxy.root(sc.ParamHash));
	}

	hname(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHname));
	}

	hnameZero(): wasmtypes.ScImmutableHname {
		return new wasmtypes.ScImmutableHname(this.proxy.root(sc.ParamHnameZero));
	}

	int64(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamInt64));
	}

	int64Zero(): wasmtypes.ScImmutableInt64 {
		return new wasmtypes.ScImmutableInt64(this.proxy.root(sc.ParamInt64Zero));
	}

	string(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamString));
	}

	stringZero(): wasmtypes.ScImmutableString {
		return new wasmtypes.ScImmutableString(this.proxy.root(sc.ParamStringZero));
	}
}

export class MutablePassTypesViewParams extends wasmtypes.ScProxy {
	address(): wasmtypes.ScMutableAddress {
		return new wasmtypes.ScMutableAddress(this.proxy.root(sc.ParamAddress));
	}

	agentID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamAgentID));
	}

	chainID(): wasmtypes.ScMutableChainID {
		return new wasmtypes.ScMutableChainID(this.proxy.root(sc.ParamChainID));
	}

	contractID(): wasmtypes.ScMutableAgentID {
		return new wasmtypes.ScMutableAgentID(this.proxy.root(sc.ParamContractID));
	}

	hash(): wasmtypes.ScMutableHash {
		return new wasmtypes.ScMutableHash(this.proxy.root(sc.ParamHash));
	}

	hname(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHname));
	}

	hnameZero(): wasmtypes.ScMutableHname {
		return new wasmtypes.ScMutableHname(this.proxy.root(sc.ParamHnameZero));
	}

	int64(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamInt64));
	}

	int64Zero(): wasmtypes.ScMutableInt64 {
		return new wasmtypes.ScMutableInt64(this.proxy.root(sc.ParamInt64Zero));
	}

	string(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamString));
	}

	stringZero(): wasmtypes.ScMutableString {
		return new wasmtypes.ScMutableString(this.proxy.root(sc.ParamStringZero));
	}
}
