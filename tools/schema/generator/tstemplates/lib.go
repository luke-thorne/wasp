// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package tstemplates

var libTs = map[string]string{
	// *******************************
	"lib.ts": `
$#emit importWasmLib
$#emit importSc

const exportMap: wasmlib.ScExportMap = {
	names: [
$#each func libExportName
	],
	funcs: [
$#each func libExportFunc
	],
	views: [
$#each func libExportView
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
$#each func libThunk
`,
	// *******************************
	"libExportName": `
		sc.$Kind$FuncName,
`,
	// *******************************
	"libExportFunc": `
$#if func libExportFuncThunk
`,
	// *******************************
	"libExportFuncThunk": `
		$kind$FuncName$+Thunk,
`,
	// *******************************
	"libExportView": `
$#if view libExportViewThunk
`,
	// *******************************
	"libExportViewThunk": `
		$kind$FuncName$+Thunk,
`,
	// *******************************
	"libThunk": `

function $kind$FuncName$+Thunk(ctx: wasmlib.Sc$Kind$+Context): void {
	ctx.log("$package.$kind$FuncName");
	let f = new sc.$FuncName$+Context();
$#if result initResultsDict
$#emit accessCheck
$#each mandatory requireMandatory
	sc.$kind$FuncName(ctx, f);
$#if result returnResultDict
	ctx.log("$package.$kind$FuncName ok");
}
`,
	// *******************************
	"initResultsDict": `
	const results = new wasmlib.ScDict([]);
	f.results = new sc.Mutable$FuncName$+Results(results.asProxy());
`,
	// *******************************
	"returnResultDict": `
	ctx.results(results);
`,
	// *******************************
	"requireMandatory": `
	ctx.require(f.params.$fldName().exists(), "missing mandatory $fldName");
`,
	// *******************************
	"accessCheck": `
$#set accessFinalize accessOther
$#emit caseAccess$funcAccess
$#emit $accessFinalize
`,
	// *******************************
	"caseAccess": `
$#set accessFinalize accessDone
`,
	// *******************************
	"caseAccessself": `

$#each funcAccessComment _funcAccessComment
	ctx.require(ctx.caller().equals(ctx.accountID()), "no permission");

$#set accessFinalize accessDone
`,
	// *******************************
	"caseAccesschain": `

$#each funcAccessComment _funcAccessComment
	ctx.require(ctx.caller().equals(ctx.chainOwnerID()), "no permission");

$#set accessFinalize accessDone
`,
	// *******************************
	"accessOther": `

$#each funcAccessComment _funcAccessComment
	const access = f.state.$funcAccess();
	ctx.require(access.exists(), "access not set: $funcAccess");
	ctx.require(ctx.caller().equals(access.value()), "no permission");

`,
	// *******************************
	"accessDone": `
`,
}
