package jsonrpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/google/martian/log"
	"github.com/iotaledger/wasp/contracts/native/evm"
	"github.com/iotaledger/wasp/packages/evm/evmtypes"
	"github.com/iotaledger/wasp/packages/kv/dict"
)

func (e *EVMChain) ChainConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID: big.NewInt(int64(e.chainID)),
	}
}

func (e *EVMChain) ChainDb() ethdb.Database {
	return nil
}

func (e *EVMChain) Engine() consensus.Engine {
	return nil
}

func (e *EVMChain) GetTransaction(_ context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	return e.getTransactionBy(evm.FuncGetTransactionByHash.Name, dict.Dict{
		evm.FieldTransactionHash: hash.Bytes(),
	})
}

// GetHeader returns the hash corresponding to their hash.
func (e *EVMChain) GetHeader(hash common.Hash, blockNumber uint64) *types.Header {
	return nil
}

func (e *EVMChain) HeaderByNumber(ctx context.Context, blockNumber rpc.BlockNumber) (*types.Header, error) {
	ret, err := e.backend.CallView(e.contractName, evm.FuncGetHeaderByNumber.Name, paramsWithOptionalBlockNumber(parseBlockNumber(blockNumber), nil))
	if err != nil {
		return nil, err
	}

	if !ret.MustHas(evm.FieldResult) {
		return nil, nil
	}

	header, err := evmtypes.DecodeHeader(ret.MustGet(evm.FieldResult))
	if err != nil {
		return nil, err
	}
	return header, nil

}

func (e *EVMChain) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	ret, err := e.backend.CallView(e.contractName, evm.FuncGetHeaderByHash.Name, dict.Dict{
		evm.FieldBlockHash: hash.Bytes(),
	})
	if err != nil {
		return nil, err
	}

	if !ret.MustHas(evm.FieldResult) {
		return nil, nil
	}

	header, err := evmtypes.DecodeHeader(ret.MustGet(evm.FieldResult))
	if err != nil {
		return nil, err
	}
	return header, nil
}

func (e *EVMChain) RPCGasCap() uint64 {
	return evm.GasLimitDefault
}

// https://github.com/ethereum/go-ethereum/blob/3038e480f5297b0e80196c156d9f9b45fd86a0bf/eth/state_accessor.go

// StateAtBlock retrieves the state database associated with a certain block.
// If no state is locally available for the given block, a number of blocks
// are attempted to be reexecuted to generate the desired state. The optional
// base layer statedb can be passed then it's regarded as the statedb of the
// parent block.
// Parameters:
// - block: The block for which we want the state (== state at the stateRoot of the parent)
// - reexec: The maximum number of blocks to reprocess trying to obtain the desired state
// - base: If the caller is tracing multiple blocks, the caller can provide the parent state
//         continuously from the callsite.
// - checklive: if true, then the live 'blockchain' state database is used. If the caller want to
//        perform Commit or other 'save-to-disk' changes, this should be set to false to avoid
//        storing trash persistently
// func(ctx context.Context, block *github.com/ethereum/go-ethereum/core/types.Block, reexec uint64, base *github.com/ethereum/go-ethereum/core/state.StateDB, checkLive bool) (*github.com/ethereum/go-ethereum/core/state.StateDB, error))
func (e *EVMChain) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (statedb *state.StateDB, err error){
	var (
		current  *types.Block
		database state.Database
		report   = true
		origin   = block.NumberU64()
	)
	// Check the live database first if we have the state fully available, use that.
	if checkLive {
		statedb, err = e.StateDBAt(block.Root())
		// statedb, err = e.blockchain.StateAt(block.Root())
		if err == nil {
			return statedb, nil
		}
	}
	if base != nil {
		// TODO: Prefer disk isn't a parameter now?
		// if preferDisk {
		// 	// Create an ephemeral trie.Database for isolating the live one. Otherwise
		// 	// the internal junks created by tracing will be persisted into the disk.
		// 	database = state.NewDatabaseWithConfig(eth.chainDb, &trie.Config{Cache: 16})
		// 	if statedb, err = state.New(block.Root(), database, nil); err == nil {
		// 		log.Info("Found disk backend for state trie", "root", block.Root(), "number", block.Number())
		// 		return statedb, nil
		// 	}
		// }
		// The optional base statedb is given, mark the start point as parent block
		statedb, database, report = base, base.Database(), false
		current, err = e.BlockByHash(context.TODO(), block.ParentHash())
		if err != nil {
			return nil, err
		}
		// current = eth.blockchain.GetBlock(block.ParentHash(), block.NumberU64()-1)
	} else {
		// Otherwise try to reexec blocks until we find a state or reach our limit
		current = block

		// Create an ephemeral trie.Database for isolating the live one. Otherwise
		// the internal junks created by tracing will be persisted into the disk.
		database = state.NewDatabaseWithConfig(e.ChainDb(), &trie.Config{Cache: 16})
		// database = state.NewDatabaseWithConfig(eth.chainDb, &trie.Config{Cache: 16})

		// If we didn't check the dirty database, do check the clean one, otherwise
		// we would rewind past a persisted block (specific corner case is chain
		// tracing from the genesis).
		if !checkLive {
			statedb, err = state.New(current.Root(), database, nil)
			if err == nil {
				return statedb, nil
			}
		}
		// Database does not have the state for the given block, try to regenerate
		for i := uint64(0); i < reexec; i++ {
			if current.NumberU64() == 0 {
				return nil, errors.New("genesis state is missing")
			}
			parent, err := e.BlockByHash(context.TODO(), current.ParentHash())
			if err != nil {
				return nil, err
			}
			if parent == nil {
				return nil, fmt.Errorf("missing block %v %d", current.ParentHash(), current.NumberU64()-1)
			}
			current = parent

			statedb, err = state.New(current.Root(), database, nil)
			if err == nil {
				break
			}
		}
		if err != nil {
			switch err.(type) {
			case *trie.MissingNodeError:
				return nil, fmt.Errorf("required historical state unavailable (reexec=%d)", reexec)
			default:
				return nil, err
			}
		}
	}
	// State was available at historical point, regenerate
	var (
		start  = time.Now()
		logged time.Time
		parent common.Hash
	)
	for current.NumberU64() < origin {
		// Print progress logs if long enough time elapsed
		if time.Since(logged) > 8*time.Second && report {
			log.Infof("Regenerating historical state {block=%d,origin=%d,remaining=%d,elapsed=%d}", current.NumberU64()+1, origin, origin-current.NumberU64()-1, time.Since(start))
			// log.Info("Regenerating historical state", "block", current.NumberU64()+1, "target", origin, "remaining", origin-current.NumberU64()-1, "elapsed", time.Since(start))
			logged = time.Now()
		}
		// Retrieve the next block to regenerate and process it
		next := rpc.BlockNumber(current.NumberU64() + 1)
		if current, err = e.BlockByNumber(context.TODO(), next); current == nil || err != nil {
		// if current = eth.blockchain.GetBlockByNumber(next); current == nil {
			return nil, fmt.Errorf("block #%d not found (err=%s)", next, err.Error())
		}
		
		// TODO: what's up with this?
		// _, _, _, err := eth.blockchain.Processor().Process(current, statedb, vm.Config{})
		// if err != nil {
		// 	return nil, fmt.Errorf("processing block %d failed: %v", current.NumberU64(), err)
		// }
		// Finalize the state so any modifications are written to the trie
		// root, err := statedb.Commit(eth.blockchain.Config().IsEIP158(current.Number()))
		// TODO: Is just deleting empty objects fine?
		root, err := statedb.Commit(true)
		if err != nil {
			return nil, fmt.Errorf("stateAtBlock commit failed, number %d root %v: %w",
				current.NumberU64(), current.Root().Hex(), err)
		}
		statedb, err = state.New(root, database, nil)
		if err != nil {
			return nil, fmt.Errorf("state reset after block %d failed: %v", current.NumberU64(), err)
		}
		database.TrieDB().Reference(root, common.Hash{})
		if parent != (common.Hash{}) {
			database.TrieDB().Dereference(parent)
		}
		parent = root
	}
	if report {
		nodes, imgs := database.TrieDB().Size()
		log.Infof("Historical state regenerated {block=%d, elapsed=%d, nodes=%v, preimages=%v}", current.NumberU64(), time.Since(start), nodes, imgs)
		// log.Info("Historical state regenerated", "block", current.NumberU64(), "elapsed", time.Since(start), "nodes", nodes, "preimages", imgs)
	}
	return statedb, nil
}

// stateAtTransaction returns the execution environment of a certain transaction.
func (e *EVMChain) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error){
	// Short circuit if it's genesis block.
	if block.NumberU64() == 0 {
		return nil, vm.BlockContext{}, nil, errors.New("no transaction in genesis")
	}
	// Create the parent state database
	parent, err := e.BlockByHash(context.TODO(),block.ParentHash())
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}
	if parent == nil {
		return nil, vm.BlockContext{}, nil, fmt.Errorf("parent %#x not found", block.ParentHash())
	}
	// Lookup the statedb of parent block from the live database,
	// otherwise regenerate it on the flight.
	statedb, err := e.StateAtBlock(ctx, parent, reexec, nil, true)
	if err != nil {
		return nil, vm.BlockContext{}, nil, err
	}
	if txIndex == 0 && len(block.Transactions()) == 0 {
		return nil, vm.BlockContext{}, statedb, nil
	}
	// Recompute transactions up to the target index.
	signer := types.MakeSigner(e.ChainConfig(), block.Number())
	for idx, tx := range block.Transactions() {
		// Assemble the transaction call message and return if the requested offset
		msg, _ := tx.AsMessage(signer, block.BaseFee())
		txContext := core.NewEVMTxContext(msg)
		context := core.NewEVMBlockContext(block.Header(), e, nil)
		if idx == txIndex {
			return msg, context, statedb, nil
		}
		// Not yet the searched for transaction, execute on top of the current state
		vmenv := vm.NewEVM(context, txContext, statedb, e.ChainConfig(), vm.Config{})
		statedb.Prepare(tx.Hash(), idx)
		if _, err := core.ApplyMessage(vmenv, msg, new(core.GasPool).AddGas(tx.Gas())); err != nil {
			return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction %#x failed: %v", tx.Hash(), err)
		}
		// Ensure any modifications are committed to the state
		// Only delete empty objects if EIP158/161 (a.k.a Spurious Dragon) is in effect
		statedb.Finalise(vmenv.ChainConfig().IsEIP158(block.Number()))
	}
	return nil, vm.BlockContext{}, nil, fmt.Errorf("transaction index %d out of range for block %#x", txIndex, block.Hash())
}
