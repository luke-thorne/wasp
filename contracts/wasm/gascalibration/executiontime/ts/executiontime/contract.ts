// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmlib from "wasmlib";
import * as sc from "./index";

export class FCall {
	func: wasmlib.ScFunc;
	params: sc.MutableFParams = new sc.MutableFParams(wasmlib.ScView.nilProxy);
	results: sc.ImmutableFResults = new sc.ImmutableFResults(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScFuncCallContext) {
		this.func = new wasmlib.ScFunc(ctx, sc.HScName, sc.HFuncF);
	}
}

export class FContext {
	params: sc.ImmutableFParams = new sc.ImmutableFParams(wasmlib.paramsProxy());
	results: sc.MutableFResults = new sc.MutableFResults(wasmlib.ScView.nilProxy);
	state: sc.MutableexecutiontimeState = new sc.MutableexecutiontimeState(wasmlib.ScState.proxy());
}

export class ScFuncs {
	static f(ctx: wasmlib.ScFuncCallContext): FCall {
		const f = new FCall(ctx);
		f.params = new sc.MutableFParams(wasmlib.newCallParamsProxy(f.func));
		f.results = new sc.ImmutableFResults(wasmlib.newCallResultsProxy(f.func));
		return f;
	}
}
