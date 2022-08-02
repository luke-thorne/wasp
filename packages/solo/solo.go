// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package solo

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/trie.go/trie"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/chain/mempool"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/database/dbmanager"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/isc/coreutil"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/metrics"
	"github.com/iotaledger/wasp/packages/parameters"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/publisher"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil/testlogger"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/utxodb"
	"github.com/iotaledger/wasp/packages/vm"
	"github.com/iotaledger/wasp/packages/vm/core/coreprocessors"
	"github.com/iotaledger/wasp/packages/vm/core/governance"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"github.com/iotaledger/wasp/packages/vm/runvm"
	_ "github.com/iotaledger/wasp/packages/vm/sandbox"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
	"github.com/iotaledger/wasp/packages/wasmvm/wasmhost"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

const (
	MaxRequestsInBlock = 100
	timeLayout         = "04:05.000000000"
)

// Solo is a structure which contains global parameters of the test: one per test instance
type Solo struct {
	// instance of the test
	T                               TestContext
	logger                          *logger.Logger
	dbmanager                       *dbmanager.DBManager
	utxoDB                          *utxodb.UtxoDB
	glbMutex                        sync.RWMutex
	ledgerMutex                     sync.RWMutex
	chains                          map[isc.ChainID]*Chain
	processorConfig                 *processors.Config
	disableAutoAdjustStorageDeposit bool
	seed                            cryptolib.Seed
}

// Chain represents state of individual chain.
// There may be several parallel instances of the chain in the 'solo' test
type Chain struct {
	// Env is a pointer to the global structure of the 'solo' test
	Env *Solo

	// Name is the name of the chain
	Name string

	// StateControllerKeyPair signature scheme of the chain address, the one used to control funds owned by the chain.
	// In Solo it is Ed25519 signature scheme (in full Wasp environment is is a BLS address)
	StateControllerKeyPair *cryptolib.KeyPair
	StateControllerAddress iotago.Address

	// ChainID is the ID of the chain (in this version alias of the ChainAddress)
	ChainID *isc.ChainID

	// OriginatorPrivateKey the key pair used to create the chain (origin transaction).
	// It is a default key pair in many of Solo calls which require private key.
	OriginatorPrivateKey *cryptolib.KeyPair
	OriginatorAddress    iotago.Address
	// OriginatorAgentID is the OriginatorAddress represented in the form of AgentID
	OriginatorAgentID isc.AgentID

	// ValidatorFeeTarget is the agent ID to which all fees are accrued. By default, it is equal to OriginatorAgentID
	ValidatorFeeTarget isc.AgentID

	// State ia an interface to access virtual state of the chain: a buffered collection of key/value pairs
	State state.VirtualStateAccess
	// GlobalSync represents global atomic flag for the optimistic state reader. In Solo it has no function
	GlobalSync coreutil.ChainStateSync
	// StateReader is the read only access to the state
	StateReader state.OptimisticStateReader
	// Log is the named logger of the chain
	log *logger.Logger
	// instance of VM
	vmRunner vm.VMRunner
	// global processor cache
	proc *processors.Cache
	// related to asynchronous backlog processing
	runVMMutex sync.Mutex
	// mempool of the chain is used in Solo to mimic a real node
	mempool mempool.Mempool
	// used for non-standard VMs
	bypassStardustVM bool
}

var _ chain.ChainCore = &Chain{}

type InitOptions struct {
	AutoAdjustStorageDeposit bool
	Debug                    bool
	PrintStackTrace          bool
	Seed                     cryptolib.Seed
	Log                      *logger.Logger
}

type InitChainOptions struct {
	// optional parameters for init request call
	InitRequestParameters dict.Dict
	// optional VMRunner. Default is StardustVM
	VMRunner vm.VMRunner
	// flag forces bypassing any StardustVM ledger-dependent calls, such as init or blocklog
	// To be used with provided non-standard VMRunner
	BypassStardustVM bool
}

func defaultInitOptions() *InitOptions {
	return &InitOptions{
		Debug:                    false,
		PrintStackTrace:          false,
		Seed:                     cryptolib.Seed{},
		AutoAdjustStorageDeposit: false, // is OFF by default
	}
}

// New creates an instance of the Solo environment
// If solo is used for unit testing, 't' should be the *testing.T instance;
// otherwise it can be either nil or an instance created with NewTestContext.
func New(t TestContext, initOptions ...*InitOptions) *Solo {
	if t == nil {
		t = NewTestContext("solo")
	}
	opt := defaultInitOptions()
	if len(initOptions) > 0 {
		opt = initOptions[0]
	}
	if opt.Log == nil {
		opt.Log = testlogger.NewNamedLogger(t.Name(), timeLayout)
		if !opt.Debug {
			opt.Log = testlogger.WithLevel(opt.Log, zapcore.InfoLevel, opt.PrintStackTrace)
		}
	}

	utxoDBinitParams := utxodb.DefaultInitParams()
	ret := &Solo{
		T:                               t,
		logger:                          opt.Log,
		dbmanager:                       dbmanager.NewDBManager(opt.Log.Named("db"), true, registry.DefaultConfig()),
		utxoDB:                          utxodb.New(utxoDBinitParams),
		chains:                          make(map[isc.ChainID]*Chain),
		processorConfig:                 coreprocessors.Config(),
		disableAutoAdjustStorageDeposit: !opt.AutoAdjustStorageDeposit,
		seed:                            opt.Seed,
	}
	globalTime := ret.utxoDB.GlobalTime()
	ret.logger.Infof("Solo environment has been created: logical time: %v, time step: %v",
		globalTime.Format(timeLayout), ret.utxoDB.TimeStep())

	err := ret.processorConfig.RegisterVMType(vmtypes.WasmTime, func(binaryCode []byte) (isc.VMProcessor, error) {
		return wasmhost.GetProcessor(binaryCode, opt.Log)
	})
	require.NoError(t, err)

	publisher.Event.Attach(events.NewClosure(func(msgType string, parts []string) {
		ret.logger.Infof("solo publisher: %s %v", msgType, parts)
	}))

	return ret
}

func (env *Solo) SyncLog() {
	_ = env.logger.Sync()
}

// WithNativeContract registers a native contract so that it may be deployed
func (env *Solo) WithNativeContract(c *coreutil.ContractProcessor) *Solo {
	env.processorConfig.RegisterNativeContract(c)
	return env
}

// NewChain deploys new chain instance.
//
// If 'chainOriginator' is nil, new one is generated and utxodb.FundsFromFaucetAmount (many) base tokens are loaded from the UTXODB faucet.
// ValidatorFeeTarget will be set to OriginatorAgentID, and can be changed after initialization.
// To deploy a chain instance the following steps are performed:
//  - chain signature scheme (private key), chain address and chain ID are created
//  - empty virtual state is initialized
//  - origin transaction is created by the originator and added to the UTXODB
//  - 'init' request transaction to the 'root' contract is created and added to UTXODB
//  - backlog processing threads (goroutines) are started
//  - VM processor cache is initialized
//  - 'init' request is run by the VM. The 'root' contracts deploys the rest of the core contracts:
// Upon return, the chain is fully functional to process requests
func (env *Solo) NewChain(chainOriginator *cryptolib.KeyPair, name string, initOptions ...InitChainOptions) *Chain {
	ret, _, _ := env.NewChainExt(chainOriginator, 0, name, initOptions...)
	return ret
}

// NewChainExt returns also origin and init transactions. Used for core testing
//nolint:funlen
func (env *Solo) NewChainExt(chainOriginator *cryptolib.KeyPair, initBaseTokens uint64, name string, initOptions ...InitChainOptions) (*Chain, *iotago.Transaction, *iotago.Transaction) {
	env.logger.Debugf("deploying new chain '%s'", name)

	vmRunner := runvm.NewVMRunner()
	var initRequestParams []dict.Dict
	bypassStardustVM := false

	if len(initOptions) > 0 {
		if initOptions[0].VMRunner != nil {
			vmRunner = initOptions[0].VMRunner
		}
		if len(initOptions[0].InitRequestParameters) > 0 {
			initRequestParams = []dict.Dict{initOptions[0].InitRequestParameters}
		}
		bypassStardustVM = initOptions[0].BypassStardustVM
	}

	stateControllerKey := env.NewKeyPairFromIndex(-1) // leaving positive indices to user
	stateControllerAddr := stateControllerKey.GetPublicKey().AsEd25519Address()

	if chainOriginator == nil {
		chainOriginator = env.NewKeyPairFromIndex(-1000 + len(env.chains)) // making new originator for each new chain
		originatorAddr := chainOriginator.GetPublicKey().AsEd25519Address()
		_, err := env.utxoDB.GetFundsFromFaucet(originatorAddr)
		require.NoError(env.T, err)
	}
	originatorAddr := chainOriginator.GetPublicKey().AsEd25519Address()
	originatorAgentID := isc.NewAgentID(originatorAddr)

	outs, outIDs := env.utxoDB.GetUnspentOutputs(originatorAddr)
	originTx, chainID, err := transaction.NewChainOriginTransaction(
		chainOriginator,
		stateControllerAddr,
		stateControllerAddr,
		initBaseTokens, // will be adjusted to min storage deposit
		outs,
		outIDs,
	)
	require.NoError(env.T, err)

	anchor, _, err := transaction.GetAnchorFromTransaction(originTx)
	require.NoError(env.T, err)

	err = env.utxoDB.AddToLedger(originTx)
	require.NoError(env.T, err)
	env.AssertL1BaseTokens(originatorAddr, utxodb.FundsFromFaucetAmount-anchor.Deposit)

	env.logger.Infof("deploying new chain '%s'. ID: %s, state controller address: %s",
		name, chainID.String(), stateControllerAddr.Bech32(parameters.L1.Protocol.Bech32HRP))
	env.logger.Infof("     chain '%s'. state controller address: %s", chainID.String(), stateControllerAddr.Bech32(parameters.L1.Protocol.Bech32HRP))
	env.logger.Infof("     chain '%s'. originator address: %s", chainID.String(), originatorAddr.Bech32(parameters.L1.Protocol.Bech32HRP))

	chainlog := env.logger.Named(name)
	store := env.dbmanager.GetOrCreateKVStore(chainID)
	vs, err := state.CreateOriginState(store, chainID)
	env.logger.Infof("     chain '%s'. origin state commitment: %s", chainID.String(), trie.RootCommitment(vs.TrieNodeStore()))

	require.NoError(env.T, err)
	require.EqualValues(env.T, 0, vs.BlockIndex())
	require.True(env.T, vs.Timestamp().IsZero())

	glbSync := coreutil.NewChainStateSync().SetSolidIndex(0)

	ret := &Chain{
		Env:                    env,
		Name:                   name,
		ChainID:                chainID,
		StateControllerKeyPair: stateControllerKey,
		StateControllerAddress: stateControllerAddr,
		OriginatorPrivateKey:   chainOriginator,
		OriginatorAddress:      originatorAddr,
		OriginatorAgentID:      originatorAgentID,
		ValidatorFeeTarget:     originatorAgentID,
		State:                  vs,
		GlobalSync:             glbSync,
		StateReader:            vs.OptimisticStateReader(glbSync),
		bypassStardustVM:       bypassStardustVM,
		vmRunner:               vmRunner,
		proc:                   processors.MustNew(env.processorConfig),
		log:                    chainlog,
	}
	ret.mempool = mempool.New(chainID.AsAddress(), ret.StateReader, chainlog, metrics.DefaultChainMetrics())
	require.NoError(env.T, err)

	// creating origin transaction with the origin of the Alias chain
	outs, ids := env.utxoDB.GetUnspentOutputs(originatorAddr)
	initTx, err := transaction.NewRootInitRequestTransaction(
		ret.OriginatorPrivateKey,
		chainID,
		"'solo' testing chain",
		outs,
		ids,
		initRequestParams...,
	)
	require.NoError(env.T, err)
	require.NotNil(env.T, initTx)

	err = env.utxoDB.AddToLedger(initTx)
	require.NoError(env.T, err)

	env.glbMutex.Lock()
	env.chains[*chainID] = ret
	env.glbMutex.Unlock()

	go ret.batchLoop()

	if bypassStardustVM {
		// force skipping the init request. It is needed for non-Stardust VMs
		return ret, originTx, nil
	}
	// run the on-ledger init request for the chain
	initReq, err := env.RequestsForChain(initTx, chainID)
	require.NoError(env.T, err)

	results := ret.RunRequestsSync(initReq, "new")
	for _, res := range results {
		require.NoError(env.T, res.Receipt.Error.AsGoError())
	}
	ret.logRequestLastBlock()

	ret.log.Infof("chain '%s' deployed. Chain ID: %s", ret.Name, ret.ChainID.String())
	return ret, originTx, initTx
}

// AddToLedger adds (synchronously confirms) transaction to the UTXODB ledger. Return error if it is
// invalid or double spend
func (env *Solo) AddToLedger(tx *iotago.Transaction) error {
	return env.utxoDB.AddToLedger(tx)
}

// RequestsForChain parses the transaction and returns all requests contained in it which have chainID as the target
func (env *Solo) RequestsForChain(tx *iotago.Transaction, chainID *isc.ChainID) ([]isc.Request, error) {
	env.glbMutex.RLock()
	defer env.glbMutex.RUnlock()

	m := env.requestsByChain(tx)
	ret, ok := m[*chainID]
	if !ok {
		return nil, fmt.Errorf("chain %s does not exist", chainID.String())
	}
	return ret, nil
}

// requestsByChain parses the transaction and extracts those outputs which are interpreted as a request to a chain
func (env *Solo) requestsByChain(tx *iotago.Transaction) map[isc.ChainID][]isc.Request {
	ret, err := isc.RequestsInTransaction(tx)
	require.NoError(env.T, err)
	return ret
}

func (env *Solo) AddRequestsToChainMempool(ch *Chain, reqs []isc.Request) {
	env.glbMutex.RLock()
	defer env.glbMutex.RUnlock()
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()

	ch.mempool.ReceiveRequests(reqs...)
}

// AddRequestsToChainMempoolWaitUntilInbufferEmpty adds all the requests to the chain mempool,
// then waits for the in-buffer to be empty, before resuming VM execution
func (env *Solo) AddRequestsToChainMempoolWaitUntilInbufferEmpty(ch *Chain, reqs []isc.Request, timeout ...time.Duration) {
	env.glbMutex.RLock()
	defer env.glbMutex.RUnlock()
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()

	ch.mempool.ReceiveRequests(reqs...)
	ch.mempool.WaitInBufferEmpty(timeout...)
}

// EnqueueRequests adds requests contained in the transaction to mempools of respective target chains
func (env *Solo) EnqueueRequests(tx *iotago.Transaction) {
	env.glbMutex.RLock()
	defer env.glbMutex.RUnlock()

	requests := env.requestsByChain(tx)

	for chidArr, reqs := range requests {
		chid, err := isc.ChainIDFromBytes(chidArr[:])
		require.NoError(env.T, err)
		ch, ok := env.chains[chidArr]
		if !ok {
			env.logger.Infof("dispatching requests. Unknown chain: %s", chid.String())
			continue
		}
		ch.runVMMutex.Lock()

		ch.mempool.ReceiveRequests(reqs...)

		ch.runVMMutex.Unlock()
	}
}

func (ch *Chain) GetAnchorOutput() *isc.AliasOutputWithID {
	outs := ch.Env.utxoDB.GetAliasOutputs(ch.ChainID.AsAddress())
	require.EqualValues(ch.Env.T, 1, len(outs))
	for id, out := range outs {
		return isc.NewAliasOutputWithID(out, id.UTXOInput())
	}
	panic("unreachable")
}

// collateBatch selects requests which are not time locked
// returns batch and and 'remains unprocessed' flag
func (ch *Chain) collateBatch() []isc.Request {
	// emulating variable sized blocks
	maxBatch := MaxRequestsInBlock - rand.Intn(MaxRequestsInBlock/3)

	now := ch.Env.GlobalTime()
	ready := ch.mempool.ReadyNow(now)
	batchSize := len(ready)
	if batchSize > maxBatch {
		batchSize = maxBatch
	}
	ret := make([]isc.Request, 0)
	for _, req := range ready[:batchSize] {
		if !req.IsOffLedger() {
			if !isc.RequestIsUnlockable(req.(isc.OnLedgerRequest), ch.ChainID.AsAddress(), now) {
				continue
			}
		}
		ret = append(ret, req)
	}
	return ret
}

// batchLoop mimics behavior Wasp consensus
func (ch *Chain) batchLoop() {
	for {
		ch.Sync()
		time.Sleep(50 * time.Millisecond)
	}
}

// Sync runs all ready requests
func (ch *Chain) Sync() {
	for {
		if !ch.collateAndRunBatch() {
			return
		}
	}
}

func (ch *Chain) collateAndRunBatch() bool {
	ch.runVMMutex.Lock()
	defer ch.runVMMutex.Unlock()

	batch := ch.collateBatch()
	if len(batch) > 0 {
		results := ch.runRequestsNolock(batch, "batchLoop")
		for _, res := range results {
			if res.Receipt.Error != nil {
				ch.log.Errorf("runRequestsSync: %v", res.Receipt.Error)
			}
		}
		return true
	}
	return false
}

// BacklogLen is a thread-safe function to return size of the current backlog
func (ch *Chain) BacklogLen() int {
	mstats := ch.MempoolInfo()
	return mstats.InBufCounter - mstats.OutPoolCounter
}

func (ch *Chain) GetCandidateNodes() []*governance.AccessNodeInfo {
	// not used, just to implement ChainCore interface
	return nil
}

func (ch *Chain) GetChainNodes() []peering.PeerStatusProvider {
	// not used, just to implement ChainCore interface
	return nil
}

func (ch *Chain) GetCommitteeInfo() *chain.CommitteeInfo {
	// not used, just to implement ChainCore interface
	return nil
}

func (ch *Chain) GlobalStateSync() coreutil.ChainStateSync {
	return ch.GlobalSync
}

func (ch *Chain) StateCandidateToStateManager(state.VirtualStateAccess, *iotago.UTXOInput) {
	// not used, just to implement ChainCore interface
}

func (ch *Chain) TriggerChainTransition(*chain.ChainTransitionEventData) {
	// not used, just to implement ChainCore interface
}

func (ch *Chain) GetStateReader() state.OptimisticStateReader {
	return ch.StateReader
}

func (ch *Chain) ID() *isc.ChainID {
	return ch.ChainID
}

func (ch *Chain) Log() *logger.Logger {
	return ch.log
}

func (ch *Chain) Processors() *processors.Cache {
	return ch.proc
}

func (ch *Chain) VirtualStateAccess() state.VirtualStateAccess {
	return ch.State.Copy()
}

func (ch *Chain) EnqueueDismissChain(_ string) {
	// not used, just to implement ChainCore interface
}

func (ch *Chain) EnqueueAliasOutput(_ *isc.AliasOutputWithID) {
	// not used, just to implement ChainCore interface
}

// ---------------------------------------------

func (env *Solo) UnspentOutputs(addr iotago.Address) (iotago.OutputSet, iotago.OutputIDs) {
	allOuts, _ := env.utxoDB.GetUnspentOutputs(addr)
	ids := make(iotago.OutputIDs, len(allOuts))
	i := 0
	for id := range allOuts {
		ids[i] = id
		i++
	}
	return allOuts, ids
}

func (env *Solo) L1NFTs(addr iotago.Address) map[iotago.OutputID]*iotago.NFTOutput {
	return env.utxoDB.GetAddressNFTs(addr)
}

// L1NativeTokens returns number of native tokens contained in the given address on the UTXODB ledger
func (env *Solo) L1NativeTokens(addr iotago.Address, tokenID *iotago.NativeTokenID) *big.Int {
	assets := env.L1Assets(addr)
	return assets.AmountNativeToken(tokenID)
}

func (env *Solo) L1BaseTokens(addr iotago.Address) uint64 {
	return env.utxoDB.GetAddressBalances(addr).BaseTokens
}

// L1Assets returns all ftokens of the address contained in the UTXODB ledger
func (env *Solo) L1Assets(addr iotago.Address) *isc.FungibleTokens {
	return env.utxoDB.GetAddressBalances(addr)
}

func (env *Solo) L1Ledger() *utxodb.UtxoDB {
	return env.utxoDB
}

type NFTMintedInfo struct {
	Output   iotago.Output
	OutputID iotago.OutputID
	NFTID    iotago.NFTID
}

// MintNFTL1 mints an NFT with the `issuer` account and sends it to a `target`` account.
// base tokens in the NFT output are sent to the minimum storage deposit and are taken from the issuer account
func (env *Solo) MintNFTL1(issuer *cryptolib.KeyPair, target iotago.Address, immutableMetadata []byte) (*isc.NFT, *NFTMintedInfo, error) {
	allOuts, allOutIDs := env.utxoDB.GetUnspentOutputs(issuer.Address())

	tx, err := transaction.NewMintNFTTransaction(transaction.MintNFTTransactionParams{
		IssuerKeyPair:     issuer,
		Target:            target,
		UnspentOutputs:    allOuts,
		UnspentOutputIDs:  allOutIDs,
		ImmutableMetadata: immutableMetadata,
	})
	if err != nil {
		return nil, nil, err
	}
	err = env.AddToLedger(tx)
	if err != nil {
		return nil, nil, err
	}

	outSet, err := tx.OutputsSet()
	if err != nil {
		return nil, nil, err
	}
	for id, out := range outSet {
		// we know that the tx will only produce 1 NFT output
		if _, ok := out.(*iotago.NFTOutput); ok {
			info := &NFTMintedInfo{
				OutputID: id,
				Output:   out,
				NFTID:    iotago.NFTIDFromOutputID(id),
			}
			iscpNFT := &isc.NFT{
				ID:       info.NFTID,
				Issuer:   issuer.Address(),
				Metadata: immutableMetadata,
			}
			return iscpNFT, info, nil
		}
	}

	return nil, nil, fmt.Errorf("NFT output not found in resulting tx")
}
