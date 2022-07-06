// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmlib from "wasmlib";
import * as sc from "./index";

const exportMap: wasmlib.ScExportMap = {
	names: [
		sc.FuncForcePayout,
		sc.FuncForceReset,
		sc.FuncInit,
		sc.FuncPayWinners,
		sc.FuncPlaceBet,
		sc.FuncPlayPeriod,
		sc.ViewLastWinningNumber,
		sc.ViewRoundNumber,
		sc.ViewRoundStartedAt,
		sc.ViewRoundStatus,
	],
	funcs: [
		funcForcePayoutThunk,
		funcForceResetThunk,
		funcInitThunk,
		funcPayWinnersThunk,
		funcPlaceBetThunk,
		funcPlayPeriodThunk,
	],
	views: [
		viewLastWinningNumberThunk,
		viewRoundNumberThunk,
		viewRoundStartedAtThunk,
		viewRoundStatusThunk,
	],
};

export function on_call(index: i32): void {
	wasmlib.WasmVMHost.connect();
	wasmlib.ScExports.call(index, exportMap);
}

export function on_load(): void {
	wasmlib.WasmVMHost.connect();
	wasmlib.ScExports.export(exportMap);
}

function funcForcePayoutThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcForcePayout");
	let f = new sc.ForcePayoutContext();

	// only SC owner can restart the round forcefully
	const access = f.state.owner();
	ctx.require(access.exists(), "access not set: owner");
	ctx.require(ctx.caller().equals(access.value()), "no permission");

	sc.funcForcePayout(ctx, f);
	ctx.log("fairroulette.funcForcePayout ok");
}

function funcForceResetThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcForceReset");
	let f = new sc.ForceResetContext();

	// only SC owner can restart the round forcefully
	const access = f.state.owner();
	ctx.require(access.exists(), "access not set: owner");
	ctx.require(ctx.caller().equals(access.value()), "no permission");

	sc.funcForceReset(ctx, f);
	ctx.log("fairroulette.funcForceReset ok");
}

function funcInitThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcInit");
	let f = new sc.InitContext();
	sc.funcInit(ctx, f);
	ctx.log("fairroulette.funcInit ok");
}

function funcPayWinnersThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcPayWinners");
	let f = new sc.PayWinnersContext();

	// only SC itself can invoke this function
	ctx.require(ctx.caller().equals(ctx.accountID()), "no permission");

	sc.funcPayWinners(ctx, f);
	ctx.log("fairroulette.funcPayWinners ok");
}

function funcPlaceBetThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcPlaceBet");
	let f = new sc.PlaceBetContext();
	ctx.require(f.params.number().exists(), "missing mandatory number");
	sc.funcPlaceBet(ctx, f);
	ctx.log("fairroulette.funcPlaceBet ok");
}

function funcPlayPeriodThunk(ctx: wasmlib.ScFuncContext): void {
	ctx.log("fairroulette.funcPlayPeriod");
	let f = new sc.PlayPeriodContext();

	// only SC owner can update the play period
	const access = f.state.owner();
	ctx.require(access.exists(), "access not set: owner");
	ctx.require(ctx.caller().equals(access.value()), "no permission");

	ctx.require(f.params.playPeriod().exists(), "missing mandatory playPeriod");
	sc.funcPlayPeriod(ctx, f);
	ctx.log("fairroulette.funcPlayPeriod ok");
}

function viewLastWinningNumberThunk(ctx: wasmlib.ScViewContext): void {
	ctx.log("fairroulette.viewLastWinningNumber");
	let f = new sc.LastWinningNumberContext();
	const results = new wasmlib.ScDict([]);
	f.results = new sc.MutableLastWinningNumberResults(results.asProxy());
	sc.viewLastWinningNumber(ctx, f);
	ctx.results(results);
	ctx.log("fairroulette.viewLastWinningNumber ok");
}

function viewRoundNumberThunk(ctx: wasmlib.ScViewContext): void {
	ctx.log("fairroulette.viewRoundNumber");
	let f = new sc.RoundNumberContext();
	const results = new wasmlib.ScDict([]);
	f.results = new sc.MutableRoundNumberResults(results.asProxy());
	sc.viewRoundNumber(ctx, f);
	ctx.results(results);
	ctx.log("fairroulette.viewRoundNumber ok");
}

function viewRoundStartedAtThunk(ctx: wasmlib.ScViewContext): void {
	ctx.log("fairroulette.viewRoundStartedAt");
	let f = new sc.RoundStartedAtContext();
	const results = new wasmlib.ScDict([]);
	f.results = new sc.MutableRoundStartedAtResults(results.asProxy());
	sc.viewRoundStartedAt(ctx, f);
	ctx.results(results);
	ctx.log("fairroulette.viewRoundStartedAt ok");
}

function viewRoundStatusThunk(ctx: wasmlib.ScViewContext): void {
	ctx.log("fairroulette.viewRoundStatus");
	let f = new sc.RoundStatusContext();
	const results = new wasmlib.ScDict([]);
	f.results = new sc.MutableRoundStatusResults(results.asProxy());
	sc.viewRoundStatus(ctx, f);
	ctx.results(results);
	ctx.log("fairroulette.viewRoundStatus ok");
}
