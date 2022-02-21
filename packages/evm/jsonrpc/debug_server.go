// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

//go:build evm_debug
// +build evm_debug

package jsonrpc

import (
	"github.com/ethereum/go-ethereum/rpc"
)

func NewServer(evmChain *EVMChain, accountManager *AccountManager) *rpc.Server {
	rpcsrv := rpc.NewServer()
	for _, srv := range []struct {
		namespace string
		service   interface{}
	}{
		{"web3", NewWeb3Service()},
		{"net", NewNetService(evmChain.chainID)},
		{"eth", NewEthService(evmChain, accountManager)},
		{"txpool", NewTxPoolService()},
		{"debug", NewDebugService(evmChain)},
	} {
		err := rpcsrv.RegisterName(srv.namespace, srv.service)
		if err != nil {
			panic(err)
		}
	}
	return rpcsrv
}
