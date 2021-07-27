// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"fmt"
	"time"

	"github.com/iotaledger/wasp/packages/iscp/colored"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/wasp/packages/chain"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/coreutil"
	"github.com/iotaledger/wasp/packages/iscp/request"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/peering"
	"github.com/iotaledger/wasp/packages/registry"
	"github.com/iotaledger/wasp/packages/state"
	"github.com/iotaledger/wasp/packages/testutil"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/processors"
	"github.com/labstack/echo/v4"
)

// waspServices is a mock implementation of the WaspServices interface
type waspServices struct{}

func (w *waspServices) ConfigDump() map[string]interface{} {
	return map[string]interface{}{
		"foo": "bar",
	}
}

func (w *waspServices) ExploreAddressBaseURL() string {
	return "http://127.0.0.1:8081/explorer/address"
}

func (w *waspServices) NetworkProvider() peering.NetworkProvider {
	return &peeringNetworkProvider{}
}

func (w *waspServices) TrustedNetworkManager() peering.TrustedNetworkManager {
	tnm := testutil.NewTrustedNetworkManager()
	_, _ = tnm.TrustPeer(ed25519.GenerateKeyPair().PublicKey, "some:254")
	_, _ = tnm.TrustPeer(ed25519.GenerateKeyPair().PublicKey, "")
	return tnm
}

func (w *waspServices) GetChain(_ *iscp.ChainID) chain.ChainCore {
	return &mockChain{}
}

func (w *waspServices) GetChainRecords() ([]*registry.ChainRecord, error) {
	r, _ := w.GetChainRecord(iscp.RandomChainID())
	return []*registry.ChainRecord{r}, nil
}

func (w *waspServices) GetChainRecord(chainID *iscp.ChainID) (*registry.ChainRecord, error) {
	return &registry.ChainRecord{
		ChainID: chainID,
		Active:  true,
	}, nil
}

func (w *waspServices) GetChainState(_ *iscp.ChainID) (*ChainState, error) {
	return &ChainState{
		Index:             1,
		Hash:              hashing.RandomHash(nil),
		Timestamp:         0,
		ApprovingOutputID: ledgerstate.OutputID{},
	}, nil
}

type peeringNetworkProvider struct{}

func (p *peeringNetworkProvider) Run(_ <-chan struct{}) {
	panic("not implemented")
}

func (p *peeringNetworkProvider) Self() peering.PeerSender {
	return &peeringNode{}
}

func (p *peeringNetworkProvider) PeerGroup(_ []string) (peering.GroupProvider, error) {
	panic("not implemented")
}

// Domain creates peering.PeerDomainProvider.
func (p *peeringNetworkProvider) PeerDomain(_ []string) (peering.PeerDomainProvider, error) {
	panic("not implemented")
}

func (p *peeringNetworkProvider) Attach(_ *peering.PeeringID, _ func(recv *peering.RecvEvent)) interface{} {
	panic("not implemented")
}

func (p *peeringNetworkProvider) Detach(_ interface{}) {
	panic("not implemented")
}

func (p *peeringNetworkProvider) PeerByNetID(_ string) (peering.PeerSender, error) {
	panic("not implemented")
}

func (p *peeringNetworkProvider) PeerByPubKey(_ *ed25519.PublicKey) (peering.PeerSender, error) {
	panic("not implemented")
}

func (p *peeringNetworkProvider) PeerStatus() []peering.PeerStatusProvider {
	return []peering.PeerStatusProvider{
		&peeringNode{},
		&peeringNode{},
		&peeringNode{},
	}
}

type peeringNode struct{}

func (p *peeringNode) IsInbound() bool {
	return false
}

func (p *peeringNode) NumUsers() int {
	return 1
}

func (p *peeringNode) NetID() string {
	return "127.0.0.1:4000"
}

func (p *peeringNode) PubKey() *ed25519.PublicKey {
	panic("not implemented")
}

func (p *peeringNode) SendMsg(_ *peering.PeerMessage) {
	panic("not implemented")
}

func (p *peeringNode) IsAlive() bool {
	return true
}

func (p *peeringNode) Await(_ time.Duration) error {
	panic("not implemented")
}

// Close releases the reference to the peer, this informs the network
// implementation, that it can disconnect, cleanup resources, etc.
// You need to get new reference to the peer (PeerSender) to use it again.
func (p *peeringNode) Close() {
	panic("not implemented")
}

func (w *waspServices) CallView(ch chain.ChainCore, hname iscp.Hname, fname string, _ dict.Dict) (dict.Dict, error) {
	chainID := ch.ID()

	contract := &root.ContractRecord{
		ProgramHash:  hashing.RandomHash(nil),
		Description:  "mock contract",
		Name:         "mock",
		OwnerFee:     42,
		ValidatorFee: 1,
		Creator:      iscp.NewRandomAgentID(),
	}

	switch {
	case hname == root.Contract.Hname() && fname == root.FuncGetChainInfo.Name:
		ret := dict.New()
		ret.Set(root.VarChainID, codec.EncodeChainID(*chainID))
		ret.Set(root.VarChainOwnerID, codec.EncodeAgentID(iscp.NewRandomAgentID()))
		ret.Set(root.VarDescription, codec.EncodeString("mock chain"))
		ret.Set(root.VarFeeColor, codec.EncodeColor(colored.Color{}))
		ret.Set(root.VarDefaultOwnerFee, codec.EncodeInt64(42))
		ret.Set(root.VarDefaultValidatorFee, codec.EncodeInt64(42))

		dst := collections.NewMap(ret, root.VarContractRegistry)
		for i := 0; i < 5; i++ {
			dst.MustSetAt(iscp.Hname(uint32(i)).Bytes(), root.EncodeContractRecord(contract))
		}
		return ret, nil

	case hname == root.Contract.Hname() && fname == root.FuncFindContract.Name:
		ret := dict.New()
		ret.Set(root.VarData, root.EncodeContractRecord(contract))
		return ret, nil

	case hname == accounts.Contract.Hname() && fname == accounts.FuncViewAccounts.Name:
		ret := dict.New()
		ret.Set(kv.Key(iscp.NewRandomAgentID().Bytes()), []byte{})
		return ret, nil

	case hname == accounts.Contract.Hname() && fname == accounts.FuncViewTotalAssets.Name:
		return accounts.EncodeBalances(colored.NewBalancesForIotas(42)), nil

	case hname == accounts.Contract.Hname() && fname == accounts.FuncViewBalance.Name:
		return accounts.EncodeBalances(colored.NewBalancesForIotas(42)), nil

	case hname == blob.Contract.Hname() && fname == blob.FuncListBlobs.Name:
		ret := dict.New()
		ret.Set(kv.Key(hashing.RandomHash(nil).Bytes()), blob.EncodeSize(4))
		return ret, nil

	case hname == blob.Contract.Hname() && fname == blob.FuncGetBlobInfo.Name:
		ret := dict.New()
		ret.Set(kv.Key([]byte("key")), blob.EncodeSize(4))
		return ret, nil

	case hname == blob.Contract.Hname() && fname == blob.FuncGetBlobField.Name:
		ret := dict.New()
		ret.Set(blob.ParamBytes, []byte{1, 3, 3, 7})
		return ret, nil
	case hname == blocklog.Contract.Hname() && fname == blocklog.FuncGetEventsForContract.Name:
		ret := dict.New()
		arr := collections.NewArray16(ret, blocklog.ParamEvent)
		arr.Push([]byte("some event"))
		return ret, nil
	}
	panic(fmt.Sprintf("mock view call not implemented: %s::%s", hname.String(), fname))
}

type mockChain struct{}

func (m *mockChain) Log() *logger.Logger {
	panic("implement me")
}

func (m *mockChain) GlobalStateSync() coreutil.ChainStateSync {
	panic("implement me")
}

func (m *mockChain) GetStateReader() state.OptimisticStateReader {
	panic("implement me")
}

func (m *mockChain) ID() *iscp.ChainID {
	return iscp.RandomChainID()
}

func (m *mockChain) GetCommitteeInfo() *chain.CommitteeInfo {
	return &chain.CommitteeInfo{
		Address:       ledgerstate.NewED25519Address(ed25519.PublicKey{}),
		Size:          2,
		Quorum:        1,
		QuorumIsAlive: true,
		PeerStatus: []*chain.PeerStatus{
			{
				Index:     0,
				PeeringID: "0",
				IsSelf:    true,
				Connected: true,
			},
			{
				Index:     1,
				PeeringID: "1",
				IsSelf:    false,
				Connected: true,
			},
		},
	}
}

func (m *mockChain) ReceiveMessage(_ interface{}) {
	panic("not implemented")
}

func (m *mockChain) Events() chain.ChainEvents {
	panic("not implemented")
}

func (m *mockChain) Processors() *processors.Cache {
	panic("not implemented")
}

func (m *mockChain) ReceiveTransaction(_ *ledgerstate.Transaction) {
	panic("not implemented")
}

func (m *mockChain) ReceiveInclusionState(_ ledgerstate.TransactionID, _ ledgerstate.InclusionState) {
	panic("not implemented")
}

func (m *mockChain) ReceiveState(_ *ledgerstate.AliasOutput, _ time.Time) {
	panic("not implemented")
}

func (m *mockChain) ReceiveOutput(_ ledgerstate.Output) {
	panic("not implemented")
}

func (m *mockChain) Dismiss(_ string) {
	panic("not implemented")
}

func (m *mockChain) IsDismissed() bool {
	panic("not implemented")
}

func (m *mockChain) GetRequestProcessingStatus(_ iscp.RequestID) chain.RequestProcessingStatus {
	panic("not implemented")
}

func (m *mockChain) EventRequestProcessed() *events.Event {
	panic("not implemented")
}

func (m *mockChain) ReceiveOffLedgerRequest(_ *request.OffLedger, _ string) {
	panic("not implemented")
}

func mockDashboard() (*echo.Echo, *Dashboard) {
	e := echo.New()
	d := Init(e, &waspServices{})
	return e, d
}
