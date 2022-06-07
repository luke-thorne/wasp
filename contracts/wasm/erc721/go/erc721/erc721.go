// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package erc721

import (
	"github.com/iotaledger/wasp/packages/wasmvm/wasmlib/go/wasmlib"
	"github.com/iotaledger/wasp/packages/wasmvm/wasmlib/go/wasmlib/wasmtypes"
)

// Follows ERC-721 standard as closely as possible
// https//eips.Ethereum.Org/EIPS/eip-721
// Notable changes w.R.T. ERC-721
// - tokenID is Hash instead of int256
// - balance amounts are Uint64 instead of int256
// - all address accounts are replaced with AgentID accounts
// - for consistency and to reduce confusion
//     use 'approved' when it is an AgentID
//     use 'approval' when it is a Bool

// set the required base URI, to which the base58 encoded token ID will be concatenated
const baseURI = "my/special/base/uri/"

var zero = wasmtypes.ScAgentID{}

///////////////////////////  HELPER FUNCTIONS  ////////////////////////////

// checks if caller is owner, or one of its delegated operators
func canOperate(state MutableErc721State, caller, owner wasmtypes.ScAgentID) bool {
	if caller == owner {
		return true
	}

	operators := state.ApprovedOperators().GetOperators(owner)
	return operators.GetBool(caller).Value()
}

// checks if caller is owner, or one of its delegated operators, or approved account for tokenID
func canTransfer(state MutableErc721State, caller, owner wasmtypes.ScAgentID, tokenID wasmtypes.ScHash) bool {
	if canOperate(state, caller, owner) {
		return true
	}

	controller := state.ApprovedAccounts().GetAgentID(tokenID)
	return controller.Value() == caller
}

// common code for safeTransferFrom and transferFrom
func transfer(ctx wasmlib.ScFuncContext, state MutableErc721State, from, to wasmtypes.ScAgentID, tokenID wasmtypes.ScHash) {
	tokenOwner := state.Owners().GetAgentID(tokenID)
	ctx.Require(tokenOwner.Exists(), "tokenID does not exist")

	owner := tokenOwner.Value()
	ctx.Require(canTransfer(state, ctx.Caller(), owner, tokenID),
		"not owner, operator, or approved")

	ctx.Require(owner == from, "from is not owner")
	// TODO ctx.Require(to == <check-if-is-a-valid-address> , "invalid 'to' agentid")

	balanceFrom := state.Balances().GetUint64(from)
	balanceTo := state.Balances().GetUint64(to)

	balanceFrom.SetValue(balanceFrom.Value() - 1)
	balanceTo.SetValue(balanceTo.Value() + 1)

	tokenOwner.SetValue(to)

	events := Erc721Events{}
	// TODO should probably clear this entry, but for now just set to zero
	currentApproved := state.ApprovedAccounts().GetAgentID(tokenID)
	if currentApproved.Exists() {
		currentApproved.Delete()
		events.Approval(zero, owner, tokenID)
	}

	events.Transfer(from, to, tokenID)
}

///////////////////////////  SC FUNCS  ////////////////////////////

// Gives permission to to to transfer tokenID token to another account.
// The approval is cleared when optional approval account is omitted.
// The approval will be cleared when the token is transferred.
func funcApprove(ctx wasmlib.ScFuncContext, f *ApproveContext) {
	tokenID := f.Params.TokenID().Value()
	tokenOwner := f.State.Owners().GetAgentID(tokenID)
	ctx.Require(tokenOwner.Exists(), "tokenID does not exist")
	owner := tokenOwner.Value()
	ctx.Require(canOperate(f.State, ctx.Caller(), owner), "not owner or operator")

	approved := f.Params.Approved()
	if !approved.Exists() {
		// remove approval if it exists
		currentApproved := f.State.ApprovedAccounts().GetAgentID(tokenID)
		if currentApproved.Exists() {
			currentApproved.Delete()
			f.Events.Approval(zero, owner, tokenID)
		}
		return
	}

	account := approved.Value()
	ctx.Require(owner != account, "approved account equals owner")

	f.State.ApprovedAccounts().GetAgentID(tokenID).SetValue(account)
	f.Events.Approval(account, owner, tokenID)
}

// Destroys tokenID. The approval is cleared when the token is burned.
func funcBurn(ctx wasmlib.ScFuncContext, f *BurnContext) {
	tokenID := f.Params.TokenID().Value()
	owner := f.State.Owners().GetAgentID(tokenID).Value()
	ctx.Require(owner != zero, "tokenID does not exist")
	ctx.Require(ctx.Caller() == owner, "caller is not owner")

	// remove approval if it exists
	currentApproved := f.State.ApprovedAccounts().GetAgentID(tokenID)
	if currentApproved.Exists() {
		currentApproved.Delete()
		f.Events.Approval(zero, owner, tokenID)
	}

	balance := f.State.Balances().GetUint64(owner)
	balance.SetValue(balance.Value() - 1)

	f.State.Owners().GetAgentID(tokenID).Delete()
	f.Events.Transfer(owner, zero, tokenID)
}

// Initializes the contract by setting a name and a symbol to the token collection.
func funcInit(ctx wasmlib.ScFuncContext, f *InitContext) {
	name := f.Params.Name().Value()
	symbol := f.Params.Symbol().Value()

	f.State.Name().SetValue(name)
	f.State.Symbol().SetValue(symbol)

	f.Events.Init(name, symbol)
}

// Mints tokenID and transfers it to caller as new owner.
func funcMint(ctx wasmlib.ScFuncContext, f *MintContext) {
	tokenID := f.Params.TokenID().Value()
	tokenOwner := f.State.Owners().GetAgentID(tokenID)
	ctx.Require(!tokenOwner.Exists(), "tokenID already minted")

	// save optional token uri
	tokenURI := f.Params.TokenURI()
	if tokenURI.Exists() {
		f.State.TokenURIs().GetString(tokenID).SetValue(tokenURI.Value())
	}

	owner := ctx.Caller()
	tokenOwner.SetValue(owner)
	balance := f.State.Balances().GetUint64(owner)
	balance.SetValue(balance.Value() + 1)

	f.Events.Transfer(zero, owner, tokenID)
	//if !owner.IsAddress() {
	//	// TODO interpret to as SC address and call its onERC721Received() func
	//}
}

// Safely transfers tokenID token from from to to, checking first that contract
// recipients are aware of the ERC721 protocol to prevent tokens from being forever locked.
func funcSafeTransferFrom(ctx wasmlib.ScFuncContext, f *SafeTransferFromContext) {
	from := f.Params.From().Value()
	to := f.Params.To().Value()
	tokenID := f.Params.TokenID().Value()
	transfer(ctx, f.State, from, to, tokenID)
	//if !to.IsAddress() {
	//	// TODO interpret to as SC address and call its onERC721Received() func
	//}
}

// Approve or remove operator as an operator for the caller.
func funcSetApprovalForAll(ctx wasmlib.ScFuncContext, f *SetApprovalForAllContext) {
	owner := ctx.Caller()
	operator := f.Params.Operator().Value()
	ctx.Require(owner != operator, "owner equals operator")

	approval := f.Params.Approval().Value()
	operatorsForCaller := f.State.ApprovedOperators().GetOperators(owner)
	operatorsForCaller.GetBool(operator).SetValue(approval)

	f.Events.ApprovalForAll(approval, operator, owner)
}

// Transfers tokenID token from from to to.
func funcTransferFrom(ctx wasmlib.ScFuncContext, f *TransferFromContext) {
	from := f.Params.From().Value()
	to := f.Params.To().Value()
	tokenID := f.Params.TokenID().Value()
	transfer(ctx, f.State, from, to, tokenID)
}

///////////////////////////  SC VIEWS  ////////////////////////////

// Returns the number of tokens in owner's account if the owner exists.
func viewBalanceOf(ctx wasmlib.ScViewContext, f *BalanceOfContext) {
	owner := f.Params.Owner().Value()
	balance := f.State.Balances().GetUint64(owner)
	if balance.Exists() {
		f.Results.Amount().SetValue(balance.Value())
	}
}

// Returns the approved account for tokenID token if there is one.
func viewGetApproved(ctx wasmlib.ScViewContext, f *GetApprovedContext) {
	tokenID := f.Params.TokenID().Value()
	approved := f.State.ApprovedAccounts().GetAgentID(tokenID)
	if approved.Exists() {
		f.Results.Approved().SetValue(approved.Value())
	}
}

// Returns if the operator is allowed to manage all the assets of owner.
func viewIsApprovedForAll(ctx wasmlib.ScViewContext, f *IsApprovedForAllContext) {
	owner := f.Params.Owner().Value()
	operator := f.Params.Operator().Value()
	operators := f.State.ApprovedOperators().GetOperators(owner)
	approval := operators.GetBool(operator)
	if approval.Exists() {
		f.Results.Approval().SetValue(approval.Value())
	}
}

// Returns the token collection name.
func viewName(ctx wasmlib.ScViewContext, f *NameContext) {
	f.Results.Name().SetValue(f.State.Name().Value())
}

// Returns the owner of the tokenID token if the token exists.
func viewOwnerOf(ctx wasmlib.ScViewContext, f *OwnerOfContext) {
	tokenID := f.Params.TokenID().Value()
	owner := f.State.Owners().GetAgentID(tokenID)
	if owner.Exists() {
		f.Results.Owner().SetValue(owner.Value())
	}
}

// Returns the token collection symbol.
func viewSymbol(ctx wasmlib.ScViewContext, f *SymbolContext) {
	f.Results.Symbol().SetValue(f.State.Symbol().Value())
}

// Returns the Uniform Resource Identifier (URI) for tokenID token if the token exists.
func viewTokenURI(ctx wasmlib.ScViewContext, f *TokenURIContext) {
	tokenID := f.Params.TokenID()
	if tokenID.Exists() {
		tokenURI := baseURI + tokenID.String()
		savedURI := f.State.TokenURIs().GetString(tokenID.Value())
		if savedURI.Exists() {
			tokenURI = savedURI.Value()
		}
		f.Results.TokenURI().SetValue(tokenURI)
	}
}
