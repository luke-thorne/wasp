// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// implementation of ERC-20 smart contract for ISCP
// following https://ethereum.org/en/developers/tutorials/understand-the-erc-20-token-smart-contract/

use wasmlib::*;

use crate::*;

// Sets the allowance value for delegated account
// inputs:
//  - PARAM_DELEGATION: agentID
//  - PARAM_AMOUNT: u64
pub fn func_approve(ctx: &ScFuncContext, f: &ApproveContext) {
    let delegation = f.params.delegation().value();
    let amount = f.params.amount().value();

    // all allowances are in the map under the name of he owner
    let allowances = f.state.all_allowances().get_allowances_for_agent(&ctx.caller());
    allowances.get_uint64(&delegation).set_value(amount);
    f.events.approval(amount, &ctx.caller(), &delegation);
}

// on_init is a constructor entry point. It initializes the smart contract with the
// initial value of the token supply and the owner of that supply
// - input:
//   -- PARAM_SUPPLY must be nonzero positive integer. Mandatory
//   -- PARAM_CREATOR is the AgentID where initial supply is placed. Mandatory
pub fn func_init(ctx: &ScFuncContext, f: &InitContext) {
    let supply = f.params.supply().value();
    ctx.require(supply > 0, "erc20.on_init.fail: wrong 'supply' parameter");
    f.state.supply().set_value(supply);

    // we cannot use 'caller' here because on_init is always called from the 'root'
    // so, owner of the initial supply must be provided as a parameter PARAM_CREATOR to constructor (on_init)
    // assign the whole supply to creator
    let creator = f.params.creator().value();
    f.state.balances().get_uint64(&creator).set_value(supply);

    let t = "erc20.on_init.success. Supply: ".to_string() + &supply.to_string() +
        &", creator:".to_string() + &creator.to_string();
    ctx.log(&t);
}

// transfer moves tokens from caller's account to target account
// This function emits the Transfer event.
// Input:
// - PARAM_ACCOUNT: agentID
// - PARAM_AMOUNT: u64
pub fn func_transfer(ctx: &ScFuncContext, f: &TransferContext) {
    let amount = f.params.amount().value();

    let balances = f.state.balances();
    let source_agent = ctx.caller();
    let source_balance = balances.get_uint64(&source_agent);
    ctx.require(source_balance.value() >= amount, "erc20.transfer.fail: not enough funds");

    let target_agent = f.params.account().value();
    let target_balance = balances.get_uint64(&target_agent);
    source_balance.set_value(source_balance.value() - amount);
    target_balance.set_value(target_balance.value() + amount);

    f.events.transfer(amount, &source_agent, &target_agent);
}

// Moves the amount of tokens from sender to recipient using the allowance mechanism.
// Amount is then deducted from the caller’s allowance.
// This function emits the Transfer event.
// Input:
// - PARAM_ACCOUNT: agentID   the spender
// - PARAM_RECIPIENT: agentID   the target
// - PARAM_AMOUNT: u64
pub fn func_transfer_from(ctx: &ScFuncContext, f: &TransferFromContext) {
    // validate parameters
    let amount = f.params.amount().value();

    // allowances are in the map under the name of the account
    let source_agent = f.params.account().value();
    let allowances = f.state.all_allowances().get_allowances_for_agent(&source_agent);
    let allowance = allowances.get_uint64(&ctx.caller());
    ctx.require(allowance.value() >= amount, "erc20.transfer_from.fail: not enough allowance");

    let balances = f.state.balances();
    let source_balance = balances.get_uint64(&source_agent);
    ctx.require(source_balance.value() >= amount, "erc20.transfer_from.fail: not enough funds");

    let target_agent = f.params.recipient().value();
    let target_balance = balances.get_uint64(&target_agent);

    source_balance.set_value(source_balance.value() - amount);
    target_balance.set_value(target_balance.value() + amount);
    allowance.set_value(allowance.value() - amount);

    f.events.transfer(amount, &source_agent, &target_agent);
}

// the view returns max number of tokens the owner PARAM_ACCOUNT of the account
// allowed to retrieve to another party PARAM_DELEGATION
// Input:
// - PARAM_ACCOUNT: agentID
// - PARAM_DELEGATION: agentID
// Output:
// - PARAM_AMOUNT: u64
pub fn view_allowance(_ctx: &ScViewContext, f: &AllowanceContext) {
    // all allowances of the address 'owner' are stored in the map of the same name
    let allowances = f.state.all_allowances().get_allowances_for_agent(&f.params.account().value());
    let allow = allowances.get_uint64(&f.params.delegation().value()).value();
    f.results.amount().set_value(allow);
}

// the view returns balance of the token held in the account
// Input:
// - PARAM_ACCOUNT: agentID
pub fn view_balance_of(_ctx: &ScViewContext, f: &BalanceOfContext) {
    let balances = f.state.balances();
    let balance = balances.get_uint64(&f.params.account().value());
    f.results.amount().set_value(balance.value());
}

// the view returns total supply set when creating the contract (a constant).
// Output:
// - PARAM_SUPPLY: u64
pub fn view_total_supply(_ctx: &ScViewContext, f: &TotalSupplyContext) {
    f.results.supply().set_value(f.state.supply().value());
}
