// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmtypes from "wasmlib/wasmtypes";

export const ScName        = "fairauction";
export const ScDescription = "Decentralized auction to securely sell NFTs to the highest bidder";
export const HScName       = new wasmtypes.ScHname(0x1b5c43b1);

export const ParamDescription = "description";
export const ParamDuration    = "duration";
export const ParamMinimumBid  = "minimumBid";
export const ParamNft         = "nft";
export const ParamOwner       = "owner";
export const ParamOwnerMargin = "ownerMargin";

export const ResultBidders       = "bidders";
export const ResultCreator       = "creator";
export const ResultDeposit       = "deposit";
export const ResultDescription   = "description";
export const ResultDuration      = "duration";
export const ResultHighestBid    = "highestBid";
export const ResultHighestBidder = "highestBidder";
export const ResultMinimumBid    = "minimumBid";
export const ResultNft           = "nft";
export const ResultOwnerMargin   = "ownerMargin";
export const ResultWhenStarted   = "whenStarted";

export const StateAuctions    = "auctions";
export const StateBidderList  = "bidderList";
export const StateBids        = "bids";
export const StateOwner       = "owner";
export const StateOwnerMargin = "ownerMargin";

export const FuncFinalizeAuction = "finalizeAuction";
export const FuncInit            = "init";
export const FuncPlaceBid        = "placeBid";
export const FuncSetOwnerMargin  = "setOwnerMargin";
export const FuncStartAuction    = "startAuction";
export const ViewGetAuctionInfo  = "getAuctionInfo";

export const HFuncFinalizeAuction = new wasmtypes.ScHname(0x8d534ddc);
export const HFuncInit            = new wasmtypes.ScHname(0x1f44d644);
export const HFuncPlaceBid        = new wasmtypes.ScHname(0x9bd72fa9);
export const HFuncSetOwnerMargin  = new wasmtypes.ScHname(0x1774461a);
export const HFuncStartAuction    = new wasmtypes.ScHname(0xd5b7bacb);
export const HViewGetAuctionInfo  = new wasmtypes.ScHname(0xd1f16936);
