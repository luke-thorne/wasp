package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/spf13/pflag"
)

func check(err error) {
	if err != nil {
		fmt.Printf("[%s] error: %s\n", os.Args[0], err)
		os.Exit(1)
	}
}

func usage(flags *pflag.FlagSet) {
	fmt.Printf("Usage: %s [init <path>|start] [options]\n", os.Args[0])
	flags.PrintDefaults()
	os.Exit(1)
}

//nolint:funlen
func main() {
	commonFlags := pflag.NewFlagSet("common flags", pflag.ExitOnError)

	templatesPath := commonFlags.StringP("templates-path", "t", ".", "Where to find alternative wasp & goshimmer config.json templates (optional)")

	config := cluster.DefaultConfig()

	commonFlags.IntVarP(&config.Wasp.NumNodes, "num-nodes", "n", config.Wasp.NumNodes, "Amount of wasp nodes")
	commonFlags.IntVarP(&config.Wasp.FirstAPIPort, "first-api-port", "a", config.Wasp.FirstAPIPort, "First wasp API port")
	commonFlags.IntVarP(&config.Wasp.FirstPeeringPort, "first-peering-port", "p", config.Wasp.FirstPeeringPort, "First wasp Peering port")
	commonFlags.IntVarP(&config.Wasp.FirstNanomsgPort, "first-nanomsg-port", "u", config.Wasp.FirstNanomsgPort, "First wasp nanomsg (publisher) port")
	commonFlags.IntVarP(&config.Wasp.FirstDashboardPort, "first-dashboard-port", "h", config.Wasp.FirstDashboardPort, "First wasp dashboard port")
	commonFlags.IntVarP(&config.Goshimmer.APIPort, "goshimmer-api-port", "i", config.Goshimmer.APIPort, "Goshimmer API port")
	commonFlags.BoolVarP(&config.Goshimmer.UseProvidedNode, "goshimmer-use-provided-node", "g", config.Goshimmer.UseProvidedNode, "If false (default), a mocked version of Goshimmer will be used")
	commonFlags.IntVarP(&config.Goshimmer.TxStreamPort, "goshimmer-txport", "gp", config.Goshimmer.TxStreamPort, "Goshimmer port")
	commonFlags.StringVarP(&config.Goshimmer.Hostname, "goshimmer-hostname", "gh", config.Goshimmer.Hostname, "Goshimmer hostname")
	commonFlags.IntVarP(&config.FaucetPoWTarget, "goshimmer-faucet-pow", "w", config.FaucetPoWTarget, "Faucet PoW target")

	if len(os.Args) < 2 {
		usage(commonFlags)
	}

	switch os.Args[1] {
	case "init":
		flags := pflag.NewFlagSet("init", pflag.ExitOnError)
		forceRemove := flags.BoolP("force", "f", false, "Force removing cluster directory if it exists")
		flags.AddFlagSet(commonFlags)

		err := flags.Parse(os.Args[2:])
		check(err)

		if flags.NArg() != 1 {
			fmt.Printf("Usage: %s init <path> [options]\n", os.Args[0])
			flags.PrintDefaults()
			os.Exit(1)
		}

		dataPath := flags.Arg(0)
		err = cluster.New("cluster", config).InitDataPath(*templatesPath, dataPath, *forceRemove, nil)
		check(err)

	case "start":
		flags := pflag.NewFlagSet("start", pflag.ExitOnError)
		disposable := flags.BoolP("disposable", "d", false, "If set, run a disposable cluster in a temporary directory (no need for init, automatically removed when stopped)")
		flags.AddFlagSet(commonFlags)

		err := flags.Parse(os.Args[2:])
		check(err)

		if flags.NArg() > 1 {
			fmt.Printf("Usage: %s start [path] [options]\n", os.Args[0])
			flags.PrintDefaults()
			os.Exit(1)
		}

		dataPath := "."
		if flags.NArg() == 1 {
			if *disposable {
				check(fmt.Errorf("[path] and -d are mutually exclusive"))
			}
			dataPath = flags.Arg(0)
		} else if *disposable {
			dataPath, err = ioutil.TempDir(os.TempDir(), "wasp-cluster-*")
			check(err)
		}

		if !*disposable {
			exists, err := cluster.ConfigExists(dataPath)
			check(err)
			if !exists {
				check(fmt.Errorf("%s/cluster.json not found. Call `%s init` first", dataPath, os.Args[0]))
			}

			config, err = cluster.LoadConfig(dataPath)
			check(err)
		}

		clu := cluster.New("wasp-cluster", config)

		if *disposable {
			check(clu.InitDataPath(*templatesPath, dataPath, true, nil))
			defer os.RemoveAll(dataPath)
		}

		err = clu.Start(dataPath)
		check(err)
		fmt.Printf("-----------------------------------------------------------------\n")
		fmt.Printf("           The cluster started\n")
		fmt.Printf("-----------------------------------------------------------------\n")

		waitCtrlC()
		clu.Wait()

	default:
		usage(commonFlags)
	}
}

func waitCtrlC() {
	fmt.Printf("[%s] Press CTRL-C to stop\n", os.Args[0])
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
}
