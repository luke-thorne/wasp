// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

#![allow(dead_code)]
#![allow(unused_imports)]

use crate::coreaccounts::*;
use crate::*;

#[derive(Clone)]
pub struct ImmutableFoundryCreateNewParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableFoundryCreateNewParams {
    pub fn token_scheme(&self) -> ScImmutableBytes {
		ScImmutableBytes::new(self.proxy.root(PARAM_TOKEN_SCHEME))
	}
}

#[derive(Clone)]
pub struct MutableFoundryCreateNewParams {
	pub(crate) proxy: Proxy,
}

impl MutableFoundryCreateNewParams {
    pub fn token_scheme(&self) -> ScMutableBytes {
		ScMutableBytes::new(self.proxy.root(PARAM_TOKEN_SCHEME))
	}
}

#[derive(Clone)]
pub struct ImmutableFoundryDestroyParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableFoundryDestroyParams {
    pub fn foundry_sn(&self) -> ScImmutableUint32 {
		ScImmutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}
}

#[derive(Clone)]
pub struct MutableFoundryDestroyParams {
	pub(crate) proxy: Proxy,
}

impl MutableFoundryDestroyParams {
    pub fn foundry_sn(&self) -> ScMutableUint32 {
		ScMutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}
}

#[derive(Clone)]
pub struct ImmutableFoundryModifySupplyParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableFoundryModifySupplyParams {
    pub fn destroy_tokens(&self) -> ScImmutableBool {
		ScImmutableBool::new(self.proxy.root(PARAM_DESTROY_TOKENS))
	}

    pub fn foundry_sn(&self) -> ScImmutableUint32 {
		ScImmutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}

    pub fn supply_delta_abs(&self) -> ScImmutableBigInt {
		ScImmutableBigInt::new(self.proxy.root(PARAM_SUPPLY_DELTA_ABS))
	}
}

#[derive(Clone)]
pub struct MutableFoundryModifySupplyParams {
	pub(crate) proxy: Proxy,
}

impl MutableFoundryModifySupplyParams {
    pub fn destroy_tokens(&self) -> ScMutableBool {
		ScMutableBool::new(self.proxy.root(PARAM_DESTROY_TOKENS))
	}

    pub fn foundry_sn(&self) -> ScMutableUint32 {
		ScMutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}

    pub fn supply_delta_abs(&self) -> ScMutableBigInt {
		ScMutableBigInt::new(self.proxy.root(PARAM_SUPPLY_DELTA_ABS))
	}
}

#[derive(Clone)]
pub struct ImmutableHarvestParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableHarvestParams {
    pub fn force_minimum_base_tokens(&self) -> ScImmutableUint64 {
		ScImmutableUint64::new(self.proxy.root(PARAM_FORCE_MINIMUM_BASE_TOKENS))
	}
}

#[derive(Clone)]
pub struct MutableHarvestParams {
	pub(crate) proxy: Proxy,
}

impl MutableHarvestParams {
    pub fn force_minimum_base_tokens(&self) -> ScMutableUint64 {
		ScMutableUint64::new(self.proxy.root(PARAM_FORCE_MINIMUM_BASE_TOKENS))
	}
}

#[derive(Clone)]
pub struct ImmutableTransferAllowanceToParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableTransferAllowanceToParams {
    pub fn agent_id(&self) -> ScImmutableAgentID {
		ScImmutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}

    pub fn force_open_account(&self) -> ScImmutableBool {
		ScImmutableBool::new(self.proxy.root(PARAM_FORCE_OPEN_ACCOUNT))
	}
}

#[derive(Clone)]
pub struct MutableTransferAllowanceToParams {
	pub(crate) proxy: Proxy,
}

impl MutableTransferAllowanceToParams {
    pub fn agent_id(&self) -> ScMutableAgentID {
		ScMutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}

    pub fn force_open_account(&self) -> ScMutableBool {
		ScMutableBool::new(self.proxy.root(PARAM_FORCE_OPEN_ACCOUNT))
	}
}

#[derive(Clone)]
pub struct ImmutableAccountNFTsParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableAccountNFTsParams {
    pub fn agent_id(&self) -> ScImmutableAgentID {
		ScImmutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct MutableAccountNFTsParams {
	pub(crate) proxy: Proxy,
}

impl MutableAccountNFTsParams {
    pub fn agent_id(&self) -> ScMutableAgentID {
		ScMutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct ImmutableBalanceParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableBalanceParams {
    pub fn agent_id(&self) -> ScImmutableAgentID {
		ScImmutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct MutableBalanceParams {
	pub(crate) proxy: Proxy,
}

impl MutableBalanceParams {
    pub fn agent_id(&self) -> ScMutableAgentID {
		ScMutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct ImmutableFoundryOutputParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableFoundryOutputParams {
    pub fn foundry_sn(&self) -> ScImmutableUint32 {
		ScImmutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}
}

#[derive(Clone)]
pub struct MutableFoundryOutputParams {
	pub(crate) proxy: Proxy,
}

impl MutableFoundryOutputParams {
    pub fn foundry_sn(&self) -> ScMutableUint32 {
		ScMutableUint32::new(self.proxy.root(PARAM_FOUNDRY_SN))
	}
}

#[derive(Clone)]
pub struct ImmutableGetAccountNonceParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableGetAccountNonceParams {
    pub fn agent_id(&self) -> ScImmutableAgentID {
		ScImmutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct MutableGetAccountNonceParams {
	pub(crate) proxy: Proxy,
}

impl MutableGetAccountNonceParams {
    pub fn agent_id(&self) -> ScMutableAgentID {
		ScMutableAgentID::new(self.proxy.root(PARAM_AGENT_ID))
	}
}

#[derive(Clone)]
pub struct ImmutableNftDataParams {
	pub(crate) proxy: Proxy,
}

impl ImmutableNftDataParams {
    pub fn nft_id(&self) -> ScImmutableNftID {
		ScImmutableNftID::new(self.proxy.root(PARAM_NFT_ID))
	}
}

#[derive(Clone)]
pub struct MutableNftDataParams {
	pub(crate) proxy: Proxy,
}

impl MutableNftDataParams {
    pub fn nft_id(&self) -> ScMutableNftID {
		ScMutableNftID::new(self.proxy.root(PARAM_NFT_ID))
	}
}
