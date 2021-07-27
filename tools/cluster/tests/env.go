package tests

import (
	"flag"
	"io/ioutil"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/colored"
	"github.com/iotaledger/wasp/packages/iscp/requestargs"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/stretchr/testify/require"
)

var (
	useGo   = flag.Bool("go", false, "use Go instead of Rust")
	useWasp = flag.Bool("wasp", false, "use Wasp built-in instead of Rust")

	wallet      = initSeed()
	scOwner     = wallet.KeyPair(0)
	scOwnerAddr = ledgerstate.NewED25519Address(scOwner.PublicKey)
)

type env struct {
	t   *testing.T
	clu *cluster.Cluster
}

type chainEnv struct {
	*env
	chain *cluster.Chain
}

func newChainEnv(t *testing.T, clu *cluster.Cluster, chain *cluster.Chain) *chainEnv {
	return &chainEnv{env: &env{t: t, clu: clu}, chain: chain}
}

type contractEnv struct {
	*chainEnv
	programHash hashing.HashValue
}

type contractWithMessageCounterEnv struct {
	*contractEnv
	counter *cluster.MessageCounter
}

func initSeed() *seed.Seed {
	return seed.NewSeed()
}

// TODO detached example code
//var builtinProgramHash = map[string]string{
//	"donatewithfeedback": dwfimpl.ProgramHash,
//	"fairauction":        fairauction.ProgramHash,
//	"fairroulette":       fairroulette.ProgramHash,
//	"inccounter":         inccounter.ProgramHash,
//	"tokenregistry":      tokenregistry.ProgramHash,
//}

func (e *chainEnv) deployContract(wasmName, scDescription string, initParams map[string]interface{}) *contractEnv {
	ret := &contractEnv{chainEnv: e}

	wasmPath := "wasm/" + wasmName + "_bg.wasm"
	if *useGo {
		wasmPath = "wasm/" + wasmName + "_go.wasm"
	}

	if !*useWasp {
		wasm, err := ioutil.ReadFile(wasmPath)
		require.NoError(e.t, err)
		_, ph, err := e.chain.DeployWasmContract(wasmName, scDescription, wasm, initParams)
		require.NoError(e.t, err)
		ret.programHash = ph
		e.t.Logf("deployContract: proghash = %s\n", ph.String())
		return ret
	}
	panic("example contract disabled")
	//fmt.Println("Using Wasp built-in SC instead of Rust Wasm SC")
	//time.Sleep(time.Second)
	//hash, ok := builtinProgramHash[wasmName]
	//if !ok {
	//	return errors.New("Unknown built-in SC: " + wasmName)
	//}

	// TODO detached example contract code
	//_, err := chain.DeployContract(wasmName, examples.VMType, hash, scDescription, initParams)
	//return err
	// return nil
}

func (e *contractWithMessageCounterEnv) postRequest(contract, entryPoint iscp.Hname, tokens int, params map[string]interface{}) {
	transfer := colored.NewBalances()
	if tokens != 0 {
		transfer = colored.NewBalancesForIotas(uint64(tokens))
	}
	e.postRequestFull(contract, entryPoint, transfer, params)
}

func (e *contractWithMessageCounterEnv) postRequestFull(contract, entryPoint iscp.Hname, transfer colored.Balances, params map[string]interface{}) {
	b := colored.NewBalances()
	if transfer != nil {
		b = transfer
	}
	tx, err := e.chainClient().Post1Request(contract, entryPoint, chainclient.PostRequestParams{
		Transfer: b,
		Args:     requestargs.New().AddEncodeSimpleMany(codec.MakeDict(params)),
	})
	require.NoError(e.t, err)
	err = e.chain.CommitteeMultiClient().WaitUntilAllRequestsProcessed(e.chain.ChainID, tx, 60*time.Second)
	require.NoError(e.t, err)
	if !e.counter.WaitUntilExpectationsMet() {
		e.t.Fail()
	}
}

func setupWithNoChain(t *testing.T) *env {
	return &env{t: t, clu: newCluster(t)}
}

func setupWithChain(t *testing.T) *chainEnv {
	e := setupWithNoChain(t)
	chain, err := e.clu.DeployDefaultChain()
	require.NoError(t, err)
	return newChainEnv(e.t, e.clu, chain)
}

func setupWithContractAndMessageCounter(t *testing.T, name, description string, nrOfRequests int) *contractWithMessageCounterEnv {
	clu := newCluster(t)

	expectations := map[string]int{
		"dismissed_committee": 0,
		"state":               3 + nrOfRequests,
		//"request_out":         3 + nrOfRequests,    // not always coming from all nodes, but from quorum only
	}

	var err error

	counter, err := clu.StartMessageCounter(expectations)
	require.NoError(t, err)
	t.Cleanup(counter.Close)

	chain, err := clu.DeployDefaultChain()
	require.NoError(t, err)

	chEnv := newChainEnv(t, clu, chain)

	cEnv := chEnv.deployContract(name, description, nil)
	require.NoError(t, err)

	chEnv.requestFunds(scOwnerAddr, "client")

	return &contractWithMessageCounterEnv{contractEnv: cEnv, counter: counter}
}

func (e *chainEnv) chainClient() *chainclient.Client {
	return chainclient.New(e.clu.GoshimmerClient(), e.clu.WaspClient(0), e.chain.ChainID, scOwner)
}
