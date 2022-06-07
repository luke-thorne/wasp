// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package rstemplates

var contractRs = map[string]string{
	// *******************************
	"contract.rs": `
#![allow(dead_code)]

$#if core useCoreContract useWasmLib
use crate::*;
$#each func FuncNameCall

pub struct ScFuncs {
}

impl ScFuncs {
$#set separator $false
$#each func FuncNameForCall
}
`,
	// *******************************
	"FuncNameCall": `
$#emit setupInitFunc

pub struct $FuncName$+Call {
	pub func: Sc$initFunc$Kind,
$#if param MutableFuncNameParams
$#if result ImmutableFuncNameResults
}
`,
	// *******************************
	"MutableFuncNameParams": `
	pub params: Mutable$FuncName$+Params,
`,
	// *******************************
	"ImmutableFuncNameResults": `
	pub results: Immutable$FuncName$+Results,
`,
	// *******************************
	"FuncNameForCall": `
$#emit setupInitFunc
$#if separator newline
$#set separator $true
$#each funcComment _funcComment
    pub fn $func_name(_ctx: &dyn Sc$Kind$+CallContext) -> $FuncName$+Call {
$#if ptrs setPtrs noPtrs
    }
`,
	// *******************************
	"setPtrs": `
        let mut f = $FuncName$+Call {
            func: Sc$initFunc$Kind::new(HSC_NAME, H$KIND$+_$FUNC_NAME),
$#if param FuncNameParamsInit
$#if result FuncNameResultsInit
        };
$#if param FuncNameParamsLink
$#if result FuncNameResultsLink
        f
`,
	// *******************************
	"FuncNameParamsInit": `
            params: Mutable$FuncName$+Params { proxy: Proxy::nil() },
`,
	// *******************************
	"FuncNameResultsInit": `
            results: Immutable$FuncName$+Results { proxy: Proxy::nil() },
`,
	// *******************************
	"FuncNameParamsLink": `
        Sc$initFunc$Kind::link_params(&mut f.params.proxy, &f.func);
`,
	// *******************************
	"FuncNameResultsLink": `
        Sc$initFunc$Kind::link_results(&mut f.results.proxy, &f.func);
`,
	// *******************************
	"noPtrs": `
        $FuncName$+Call {
            func: Sc$initFunc$Kind::new(HSC_NAME, H$KIND$+_$FUNC_NAME),
        }
`,
}
