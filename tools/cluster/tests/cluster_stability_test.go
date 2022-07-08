/**
This test will test the recovery capabilities of the clusterized nodes, where some nodes can break/restart/lay dead at any time.
*/

package tests

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/stretchr/testify/require"
)

const OSWindows string = "windows"

type SabotageEnv struct {
	chainEnv      *ChainEnv
	NumValidators int
	SabotageList  []int
}

func initializeStabilityTest(t *testing.T, numValidators, clusterSize int) *SabotageEnv {
	progHash := inccounter.Contract.ProgramHash
	env := SetupWithChain(t, waspClusterOpts{nNodes: clusterSize})
	_, _, err := env.Clu.InitDKG(numValidators)

	require.NoError(t, err)

	_, _ = env.Chain.DeployContract(nativeIncCounterSCName, progHash.String(), "testing with inccounter", nil)
	waitUntil(t, env.contractIsDeployed(nativeIncCounterSCName), env.Clu.Config.AllNodes(), 50*time.Second, "contract is deployed")

	return &SabotageEnv{
		chainEnv:      env,
		NumValidators: numValidators,
		SabotageList:  make([]int, 0),
	}
}

func (e *SabotageEnv) sendRequests(numRequests int, messageDelay time.Duration) {
	for i := 0; i < numRequests; i++ {
		client := e.chainEnv.createNewClient()

		_, err := client.PostRequest(inccounter.FuncIncCounter.Name)
		require.NoError(e.chainEnv.t, err)

		time.Sleep(messageDelay)
	}
}

func (e *SabotageEnv) setSabotageValidators(breakCount int) {
	clusterSize := e.chainEnv.Clu.Config.Wasp.NumNodes

	from := clusterSize - e.NumValidators
	to := from + breakCount - 1

	e.SabotageList = util.MakeRange(from, to)
}

func (e *SabotageEnv) setSabotageAll(breakCount int) {
	from := 1
	to := from + breakCount - 1

	e.SabotageList = util.MakeRange(from, to)
}

type SabotageOption int

const (
	SabotageByKill SabotageOption = iota
	// Important: Frozen nodes need to get killed/unfrozen manually after the test is done, otherwise they stay alive after the test has finished
	SabotageByFreeze SabotageOption = iota
)

func (e *SabotageEnv) sabotageNodes(sabotageOption SabotageOption, startDelay, inBetweenDelay time.Duration) *sync.WaitGroup {
	// Give the test time to start

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		e.chainEnv.t.Log("Sabotaging the following nodes:\n")
		e.chainEnv.t.Log(e.SabotageList)

		time.Sleep(startDelay)

		for _, nodeID := range e.SabotageList {
			e.chainEnv.t.Logf("Breaking node: %v (%s)", nodeID, time.Now())

			var err error

			if sabotageOption == SabotageByKill {
				err = e.chainEnv.Clu.KillNodeProcess(nodeID)
			} else if sabotageOption == SabotageByFreeze {
				err = e.chainEnv.Clu.FreezeNode(nodeID)
			}

			require.NoError(e.chainEnv.t, err)

			time.Sleep(inBetweenDelay)
		}

		wg.Done()
	}()

	return &wg
}

func (e *SabotageEnv) unfreezeNodes() {
	for _, nodeID := range e.SabotageList {
		e.chainEnv.t.Logf("Unfreezing node %v (%s)", nodeID, time.Now())
		err := e.chainEnv.Clu.UnfreezeNode(nodeID)

		require.NoError(e.chainEnv.t, err)
	}
}

func (e *SabotageEnv) getActiveNodeList() []int {
	contains := func(x int) bool {
		for _, n := range e.SabotageList {
			if n == x {
				return true
			}
		}

		return false
	}

	activeNodeList := []int{}

	for _, n := range e.chainEnv.Clu.Config.AllNodes() {
		if !contains(n) {
			activeNodeList = append(activeNodeList, n)
		}
	}

	return activeNodeList
}

func TestSuccessfulIncCounterIncreaseWithoutInstability(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	const clusterSize = 8
	const numValidators = 6
	const numRequests = 35

	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.sendRequests(numRequests, time.Millisecond*250)
	waitUntil(t, env.chainEnv.counterEquals(int64(numRequests)), env.chainEnv.Clu.Config.AllNodes(), 120*time.Second, "incCounter matches expectation")
}

func TestSuccessfulIncCounterIncreaseWithMildInstability(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	testutil.RunHeavy(t)

	const clusterSize = 10
	const numValidators = 9
	const numBrokenNodes = 2
	const numRequests = 35

	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.setSabotageValidators(numBrokenNodes)

	wg := env.sabotageNodes(SabotageByKill, 4*time.Second, 1*time.Second)
	env.sendRequests(numRequests, time.Millisecond*250)

	wg.Wait()

	waitUntil(t, env.chainEnv.counterEquals(int64(numRequests)), env.getActiveNodeList(), 120*time.Second, "incCounter matches expectation")
}

func runTestFailsIncCounterIncreaseAsQuorumNotMet(t *testing.T, clusterSize, numValidators, numBrokenNodes, numRequests int) {
	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.setSabotageAll(numBrokenNodes)

	wg := env.sabotageNodes(SabotageByKill, 5*time.Second, 500*time.Millisecond)
	env.sendRequests(numRequests, time.Millisecond*250)

	wg.Wait()
	// quorum is not met, incCounter should not equal numRequests
	time.Sleep(time.Second * 25)
	counter := env.chainEnv.getCounter(nativeIncCounterSCHname)
	require.NotEqual(t, numRequests, int(counter))
}

func TestFailsIncCounterIncreaseAsQuorumNotMet(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	t.Run("cluster=3,numValidators=2,numBrokenNodes=2,req=35", func(t *testing.T) {
		const clusterSize = 3
		const numValidators = 2
		const numBrokenNodes = 2
		const numRequests = 35

		runTestFailsIncCounterIncreaseAsQuorumNotMet(t, clusterSize, numValidators, numBrokenNodes, numRequests)
	})

	t.Run("cluster=14,numValidators=12,numBrokenNodes=11,req=35", func(t *testing.T) {
		testutil.RunHeavy(t)
		const clusterSize = 14
		const numValidators = 12
		const numBrokenNodes = 11
		const numRequests = 35

		runTestFailsIncCounterIncreaseAsQuorumNotMet(t, clusterSize, numValidators, numBrokenNodes, numRequests)
	})
}

func testConsenseusReconnectingNodesNoQuorum(t *testing.T, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure int) {
	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.setSabotageValidators(numBrokenNodes)

	t.Cleanup(func() {
		// This hook is just a safety measure to unfreeze all nodes when an error happens. Otherwise they stay in a zombie mode after the tests ended.
		if env != nil {
			env.unfreezeNodes()
		}
	})

	env.sendRequests(numRequestsBeforeFailure, time.Millisecond*250)
	waitUntil(t, env.chainEnv.counterEquals(int64(numRequestsBeforeFailure)), env.getActiveNodeList(), 60*time.Second, "incCounter matches expectation")

	wg := env.sabotageNodes(SabotageByFreeze, 5*time.Second, 1*time.Second)
	env.sendRequests(numRequestsAfterFailure, time.Millisecond*500)
	wg.Wait()

	t.Log("Waiting for network to settle, no incCounter increases should be logged now")
	time.Sleep(time.Second * 25)
	counter := env.chainEnv.getCounter(nativeIncCounterSCHname)
	// quorum is not met, work stops, incCounter should not equal numRequests
	require.NotEqual(env.chainEnv.t, numRequestsBeforeFailure+numRequestsAfterFailure, int(counter))

	// unfreeze nodes, after bootstrapping it is expected to reach a full quorum leading to an equal incCounter
	env.unfreezeNodes()

	waitUntil(t, env.chainEnv.counterEquals(int64(numRequestsBeforeFailure+numRequestsAfterFailure)), env.chainEnv.Clu.Config.AllNodes(), 60*time.Second, "incCounter matches expectation")
	time.Sleep(10 * time.Second) // wait for nodes to be properly shutdown before starting a new test
}

func testConsenseusReconnectingNodesHighQuorum(t *testing.T, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure int) {
	// Windows does not support freezing with SIGSTOP, we skip those for now.
	if runtime.GOOS == OSWindows {
		t.Skip()
	}

	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.setSabotageValidators(numBrokenNodes)

	t.Cleanup(func() {
		// This hook is just a safety measure to unfreeze all nodes when an error happens. Otherwise they stay in a zombie mode after the tests ended.
		if env != nil {
			env.unfreezeNodes()
		}
	})

	env.sendRequests(numRequestsBeforeFailure, time.Millisecond*250)
	waitUntil(t, env.chainEnv.counterEquals(int64(numRequestsBeforeFailure)), env.getActiveNodeList(), 60*time.Second, "incCounter matches expectation")

	wg := env.sabotageNodes(SabotageByFreeze, 5*time.Second, 1*time.Second)
	env.sendRequests(numRequestsAfterFailure, time.Millisecond*500)
	wg.Wait()

	waitUntil(t, env.chainEnv.counterEquals(int64(numRequestsBeforeFailure+numRequestsAfterFailure)), env.getActiveNodeList(), 60*time.Second, "incCounter matches expectation")
	env.unfreezeNodes()
	time.Sleep(10 * time.Second) // wait for nodes to be properly shutdown before starting a new test
}

func TestSuccessfulConsenseusWithReconnectingNodes(t *testing.T) {
	/**
	* incCounter requests get sent, after reaching a matching counter value, nodes get shut down, new requests get send in parallel.
	*	If frozen nodes are below the quorum level, the incCounter count should not reach numRequests until unfrozen, otherwise the opposite is expected
	 */

	if testing.Short() {
		t.SkipNow()
	}

	// Windows does not support freezing with SIGSTOP, we skip those for now.
	if runtime.GOOS == OSWindows {
		t.Skip()
	}

	t.Run("cluster=5,numValidators=4,numBrokenNodes=4,req=35,quorum=NO", func(t *testing.T) {
		const clusterSize = 5
		const numValidators = 4
		const numBrokenNodes = 4
		const numRequestsBeforeFailure = 10
		const numRequestsAfterFailure = 25

		testConsenseusReconnectingNodesNoQuorum(t, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure)
	})

	t.Run("cluster=15,numValidators=13,numBrokenNodes=12,req=35,quorum=NO", func(t *testing.T) {
		testutil.RunHeavy(t)
		const clusterSize = 15
		const numValidators = 13
		const numBrokenNodes = 12
		const numRequestsBeforeFailure = 10
		const numRequestsAfterFailure = 25

		testConsenseusReconnectingNodesNoQuorum(t, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure)
	})

	t.Run("cluster=4,numValidators=3,numBrokenNodes=1,req=35,quorum=YES", func(t *testing.T) {
		const clusterSize = 4
		const numValidators = 3
		const numBrokenNodes = 1
		const numRequestsBeforeFailure = 10
		const numRequestsAfterFailure = 25

		testConsenseusReconnectingNodesHighQuorum(t, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure)
	})

	t.Run("cluster=12,numValidators=10,numBrokenNodes=4,req=35,quorum=YES", func(t *testing.T) {
		testutil.RunHeavy(t)
		const clusterSize = 12
		const numValidators = 10
		const numBrokenNodes = 4
		const numRequestsBeforeFailure = 10
		const numRequestsAfterFailure = 25

		testConsenseusReconnectingNodesHighQuorum(t, clusterSize, numValidators, numBrokenNodes, numRequestsBeforeFailure, numRequestsAfterFailure)
	})
}

func runTestOneFailingNodeAfterTheOther(t *testing.T, clusterSize, numValidators, numBrokenNodes, numRequestsEachStep int) {
	quorum := (2*numValidators)/3 + 1

	t.Logf("Quorum: %v", quorum)
	t.Logf("Maximum allowed broken nodes: %v", numValidators-quorum)

	requestCounter := 0
	brokenNodes := 0

	env := initializeStabilityTest(t, numValidators, clusterSize)
	env.setSabotageValidators(numBrokenNodes)

	t.Cleanup(func() {
		// This hook is just a safety measure to unfreeze all nodes when an error happens. Otherwise they stay in a zombie mode after the tests ended.
		if env != nil {
			env.unfreezeNodes()
		}
	})

	t.Logf("Nodes to break: %v", env.SabotageList)

	for _, nodeID := range env.SabotageList {
		requestCounter += numRequestsEachStep
		go env.sendRequests(numRequestsEachStep, time.Millisecond*1)

		time.Sleep(time.Millisecond * 1000)
		env.chainEnv.Clu.FreezeNode(nodeID)
		brokenNodes++

		t.Logf("on broken node: %v(%v), quorum=%v, match=%v", nodeID, brokenNodes, numValidators-quorum, brokenNodes >= numValidators-quorum)
		if brokenNodes >= numValidators-quorum {
			// Wait and validate that not all messages have arrived
			time.Sleep(15 * time.Second)

			counter := env.chainEnv.getCounter(nativeIncCounterSCHname)
			require.NotEqual(env.chainEnv.t, requestCounter, int(counter))

			break
		} else {
			t.Log("Waiting for requests to come in")
			waitUntil(t, env.chainEnv.counterEquals(int64(requestCounter)), env.getActiveNodeList(), 60*time.Second, "incCounter matches expectation")
			counter := env.chainEnv.getCounter(nativeIncCounterSCHname)

			t.Logf("Seems good? %v", counter)
		}
	}

	// Either too many nodes are down now and the process has stopped, or quorum is still fine, requiring no further interaction.

	counter := env.chainEnv.getCounter(nativeIncCounterSCHname)

	t.Logf("Counter after first iteration: %v", counter)

	if counter == int64(requestCounter) {
		return
	}

	t.Logf("Counter does not match requestCounter: %v", requestCounter)

	for _, nodeID := range env.SabotageList {
		env.chainEnv.Clu.UnfreezeNode(nodeID)

		time.Sleep(time.Second * 15)
		counter = env.chainEnv.getCounter(nativeIncCounterSCHname)

		if counter == int64(requestCounter) {
			t.Logf("After unfreezing, the counter matches the requestCounter: %v", requestCounter)

			break
		}
	}

	require.Equal(t, requestCounter, int(counter))
	time.Sleep(10 * time.Second) // wait for nodes to be properly shutdown before starting a new test
}

func TestOneFailingNodeAfterTheOther(t *testing.T) {
	// In this test one node after the other gets disabled by either killing or freezing.
	// Until quorum is unreachable, messages should all be flowing normally.

	if testing.Short() {
		t.SkipNow()
	}

	// Windows does not support freezing with SIGSTOP, we skip those for now.
	if runtime.GOOS == OSWindows {
		t.Skip()
	}
	t.Run("cluster=5,numValidators=4,numBrokenNodes=1", func(t *testing.T) {
		const clusterSize = 5
		const numValidators = 4
		const numBrokenNodes = 1
		const numRequestsEachStep = 10

		runTestOneFailingNodeAfterTheOther(t, clusterSize, numValidators, numBrokenNodes, numRequestsEachStep)
	})

	t.Run("cluster=9,numValidators=6,numBrokenNodes=1", func(t *testing.T) {
		const clusterSize = 5
		const numValidators = 4
		const numBrokenNodes = 1
		const numRequestsEachStep = 10

		runTestOneFailingNodeAfterTheOther(t, clusterSize, numValidators, numBrokenNodes, numRequestsEachStep)
	})

	t.Run("cluster=10,numValidators=9,numBrokenNodes=2", func(t *testing.T) {
		const clusterSize = 10
		const numValidators = 9
		const numBrokenNodes = 2
		const numRequestsEachStep = 10

		runTestOneFailingNodeAfterTheOther(t, clusterSize, numValidators, numBrokenNodes, numRequestsEachStep)
	})
}
