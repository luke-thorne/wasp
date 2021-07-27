package tests

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/tools/cluster"
	"github.com/stretchr/testify/require"
)

type WaspCLITest struct {
	T       *testing.T
	Cluster *cluster.Cluster
	dir     string
}

func newWaspCLITest(t *testing.T) *WaspCLITest {
	clu := newCluster(t)

	dir, err := ioutil.TempDir(os.TempDir(), "wasp-cli-test-*")
	t.Logf("Using temporary directory %s", dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	w := &WaspCLITest{
		T:       t,
		Cluster: clu,
		dir:     dir,
	}
	w.Run("set", "utxodb", "true")
	return w
}

func (w *WaspCLITest) runCmd(args []string, f func(*exec.Cmd)) []string {
	// -w: wait for requests
	// -d: debug output
	cmd := exec.Command("wasp-cli", append([]string{"-w", "-d"}, args...)...) //nolint:gosec
	cmd.Dir = w.dir

	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if f != nil {
		f(cmd)
	}

	w.T.Logf("Running: %s", strings.Join(cmd.Args, " "))
	err := cmd.Run()

	outStr, errStr := stdout.String(), stderr.String()
	if err != nil {
		require.NoError(w.T, fmt.Errorf(
			"cmd `wasp-cli %s` failed\n%w\noutput:\n%s",
			strings.Join(args, " "),
			err,
			outStr+errStr,
		))
	}
	outStr = strings.Replace(outStr, "\r", "", -1)
	outStr = strings.TrimRight(outStr, "\n")
	return strings.Split(outStr, "\n")
}

func (w *WaspCLITest) Run(args ...string) []string {
	return w.runCmd(args, nil)
}

func (w *WaspCLITest) Pipe(in []string, args ...string) []string {
	return w.runCmd(args, func(cmd *exec.Cmd) {
		cmd.Stdin = bytes.NewReader([]byte(strings.Join(in, "\n")))
	})
}

// CopyFile copies the given file into the temp directory
func (w *WaspCLITest) CopyFile(srcFile string) {
	source, err := os.Open(srcFile)
	require.NoError(w.T, err)
	defer source.Close()

	dst := path.Join(w.dir, path.Base(srcFile))
	destination, err := os.Create(dst)
	require.NoError(w.T, err)
	defer destination.Close()

	_, err = io.Copy(destination, source)
	require.NoError(w.T, err)
}

func (w *WaspCLITest) CommitteeConfig() (string, string) {
	var committee []string
	for i := 0; i < w.Cluster.Config.Wasp.NumNodes; i++ {
		committee = append(committee, fmt.Sprintf("%d", i))
	}

	quorum := 3 * w.Cluster.Config.Wasp.NumNodes / 4
	if quorum < 1 {
		quorum = 1
	}

	return "--committee=" + strings.Join(committee, ","), fmt.Sprintf("--quorum=%d", quorum)
}

func (w *WaspCLITest) Address() ledgerstate.Address {
	out := w.Run("address")
	s := regexp.MustCompile(`(?m)Address:[[:space:]]+([[:alnum:]]+)$`).FindStringSubmatch(out[1])[1] //nolint:gocritic
	addr, err := ledgerstate.AddressFromBase58EncodedString(s)
	require.NoError(w.T, err)
	return addr
}
