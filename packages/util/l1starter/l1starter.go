package l1starter

import (
	"context"
	"flag"
	"os"
	"path"

	"github.com/iotaledger/wasp/packages/nodeconn"
	"github.com/iotaledger/wasp/packages/testutil/privtangle"
	"github.com/iotaledger/wasp/packages/testutil/privtangle/privtangledefaults"
)

type L1Starter struct {
	Config             nodeconn.L1Config
	privtangleNumNodes int
	Privtangle         *privtangle.PrivTangle
}

// New sets up the CLI flags relevant to L1/privtangle configuration in the given FlagSet.
func New(flags *flag.FlagSet) *L1Starter {
	s := &L1Starter{}
	flags.StringVar(&s.Config.APIAddress, "layer1-api", "", "layer1 API address")
	flags.StringVar(&s.Config.FaucetAddress, "layer1-faucet", "", "layer1 faucet port")
	flags.BoolVar(&s.Config.UseRemotePoW, "layer1-remote-pow", false, "use remote PoW (must be enabled on the Hornet node)")
	flags.IntVar(&s.privtangleNumNodes, "privtangle-num-nodes", 2, "number of hornet nodes to be spawned in the private tangle")
	return s
}

func (s *L1Starter) PrivtangleEnabled() bool {
	return s.Config.APIAddress == "" || s.Privtangle != nil
}

// StartPrivtangleIfNecessary starts a private tangle, unless an L1 host was provided via cli flags
func (s *L1Starter) StartPrivtangleIfNecessary(logfunc privtangle.LogFunc) {
	if s.Config.APIAddress != "" {
		return
	}
	if s.Privtangle != nil {
		// restart mqtt server (to avoid some errors when running many tests in a row)
		s.Privtangle.RestartMqtt()
		return
	}
	s.Privtangle = privtangle.Start(
		context.Background(),
		path.Join(os.TempDir(), "privtangle"),
		privtangledefaults.BasePort,
		s.privtangleNumNodes,
		logfunc,
	)
	s.Config = s.Privtangle.L1Config()
}

func (s *L1Starter) Stop() {
	if s.Privtangle != nil {
		s.Privtangle.Stop()
	}
}
