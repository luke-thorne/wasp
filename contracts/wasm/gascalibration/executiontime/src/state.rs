// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

#![allow(dead_code)]
#![allow(unused_imports)]

use wasmlib::*;

use crate::*;

#[derive(Clone)]
pub struct ImmutableexecutiontimeState {
	pub(crate) proxy: Proxy,
}

impl ImmutableexecutiontimeState {
    // current owner of this smart contract
    pub fn owner(&self) -> ScImmutableAgentID {
		ScImmutableAgentID::new(self.proxy.root(STATE_OWNER))
	}
}

#[derive(Clone)]
pub struct MutableexecutiontimeState {
	pub(crate) proxy: Proxy,
}

impl MutableexecutiontimeState {
    pub fn as_immutable(&self) -> ImmutableexecutiontimeState {
		ImmutableexecutiontimeState { proxy: self.proxy.root("") }
	}

    // current owner of this smart contract
    pub fn owner(&self) -> ScMutableAgentID {
		ScMutableAgentID::new(self.proxy.root(STATE_OWNER))
	}
}
