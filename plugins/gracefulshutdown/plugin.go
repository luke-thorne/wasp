package gracefulshutdown

import (
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the graceful shutdown plugin.
const PluginName = "Graceful Shutdown"

// WaitToKillTimeInSeconds is the maximum amount of time to wait for background processes to terminate.
// After that the process is killed.
const WaitToKillTimeInSeconds = 60

var (
	log          *logger.Logger
	gracefulStop chan os.Signal
)

func Init() *node.Plugin {
	return node.NewPlugin(PluginName, nil, node.Enabled, configure)
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(PluginName)
	gracefulStop = make(chan os.Signal)

	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop

		log.Warnf("Received shutdown request - waiting (max %d) to finish processing ...", WaitToKillTimeInSeconds)

		go func() {
			start := time.Now()
			for x := range time.NewTicker(1 * time.Second).C {
				secondsSinceStart := x.Sub(start).Seconds()

				if secondsSinceStart <= WaitToKillTimeInSeconds {
					processList := ""
					runningBackgroundWorkers := daemon.GetRunningBackgroundWorkers()
					if len(runningBackgroundWorkers) >= 1 {
						processList = "(" + strings.Join(runningBackgroundWorkers, ", ") + ") "
					}
					log.Warnf("Received shutdown request - waiting (max %d seconds) to finish processing %s...", WaitToKillTimeInSeconds-int(secondsSinceStart), processList)
				} else {
					log.Error("Background processes did not terminate in time! Forcing shutdown ...")
					os.Exit(1)
				}
			}
		}()

		daemon.Shutdown()
	}()
}

// Shutdown shuts down the default daemon instance.
func Shutdown() {
	gracefulStop <- syscall.SIGINT
}

// ShutdownWithError prints out an error message and shuts down the default daemon instance.
func ShutdownWithError(err error) {
	log.Error(err)
	Shutdown()
}
