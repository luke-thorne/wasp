package dashboard

import (
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/parameters"
)

func args(args ...interface{}) []interface{} {
	return args
}

func hashref(hash hashing.HashValue) *hashing.HashValue {
	return &hash
}

func chainIDref(chID iscp.ChainID) *iscp.ChainID {
	return &chID
}

func assetID(aID []byte) []byte {
	return aID
}

func trim(max int, s string) string {
	if len(s) > max {
		s = s[:max] + "…"
	}
	// escape unprintable chars
	s = fmt.Sprintf("%q", s)
	// remove quotes
	return s[1 : len(s)-1]
}

func incUint32(n uint32) uint32 {
	return n + 1
}

func decUint32(n uint32) uint32 {
	return n - 1
}

func keyToString(k kv.Key) string {
	return string(k)
}

func bytesToString(b []byte) string {
	return string(b)
}

func anythingToString(i interface{}) string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("%v", i)
}

func formatTimestamp(ts interface{}) string {
	t, ok := ts.(time.Time)
	if !ok {
		t = time.Unix(0, ts.(int64))
	}
	return t.UTC().Format(time.RFC3339)
}

func formatTimestampOrNever(t time.Time) string {
	timestampNever := time.Time{}
	if t == timestampNever {
		return "NEVER"
	}
	return formatTimestamp(t)
}

func (d *Dashboard) exploreAddressURL(address iotago.Address) string {
	return d.wasp.ExploreAddressBaseURL() + "/" + d.addressToString(address)
}

func (d *Dashboard) addressToString(a iotago.Address) string {
	return a.Bech32(parameters.L1.Protocol.Bech32HRP)
}

func (d *Dashboard) agentIDToString(a iscp.AgentID) string {
	return a.String()
}

func (d *Dashboard) getETHAddress(a iscp.AgentID) string {
	if !d.isETHAddress(a) {
		return ""
	}

	ethAgent, _ := a.(*iscp.EthereumAddressAgentID)

	return ethAgent.EthAddress().String()
}

func (d *Dashboard) isETHAddress(a iscp.AgentID) bool {
	_, ok := a.(*iscp.EthereumAddressAgentID)

	return ok
}

func (d *Dashboard) isValidAddress(a iscp.AgentID) bool {
	addr := d.addressFromAgentID(a)

	if addr != nil {
		return true
	}

	return d.isETHAddress(a)
}

func (d *Dashboard) addressFromAgentID(a iscp.AgentID) iotago.Address {
	addr, _ := iscp.AddressFromAgentID(a)
	return addr
}
