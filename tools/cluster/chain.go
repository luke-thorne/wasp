package cluster

import (
	"bytes"
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/client/wallet/packages/seed"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/client/multiclient"
	"github.com/iotaledger/wasp/client/scclient"
	"github.com/iotaledger/wasp/contracts/native/inccounter"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/requestargs"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/blob"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/root"
	"github.com/iotaledger/wasp/packages/vm/vmtypes"
)

type Chain struct {
	Description string

	OriginatorSeed *seed.Seed

	AllPeers       []int
	CommitteeNodes []int
	Quorum         uint16
	StateAddress   ledgerstate.Address

	ChainID iscp.ChainID

	Cluster *Cluster
}

func (ch *Chain) ChainAddress() ledgerstate.Address {
	return ch.ChainID.AsAddress()
}

func (ch *Chain) CommitteeAPIHosts() []string {
	return ch.Cluster.Config.APIHosts(ch.CommitteeNodes)
}

func (ch *Chain) CommitteePeeringHosts() []string {
	return ch.Cluster.Config.PeeringHosts(ch.CommitteeNodes)
}

func (ch *Chain) AllPeeringHosts() []string {
	return ch.Cluster.Config.PeeringHosts(ch.AllPeers)
}

func (ch *Chain) AllAPIHosts() []string {
	return ch.Cluster.Config.APIHosts(ch.AllPeers)
}

func (ch *Chain) OriginatorAddress() ledgerstate.Address {
	addr := ch.OriginatorSeed.Address(0).Address()
	return addr
}

func (ch *Chain) OriginatorID() *iscp.AgentID {
	ret := iscp.NewAgentID(ch.OriginatorAddress(), 0)
	return ret
}

func (ch *Chain) OriginatorKeyPair() *ed25519.KeyPair {
	return ch.OriginatorSeed.KeyPair(0)
}

func (ch *Chain) OriginatorClient() *chainclient.Client {
	return ch.Client(ch.OriginatorKeyPair())
}

func (ch *Chain) Client(sigScheme *ed25519.KeyPair, nodeIndex ...int) *chainclient.Client {
	idx := 0
	if len(nodeIndex) == 1 {
		idx = nodeIndex[0]
	}
	return chainclient.New(
		ch.Cluster.GoshimmerClient(),
		ch.Cluster.WaspClient(idx),
		ch.ChainID,
		sigScheme,
	)
}

func (ch *Chain) SCClient(contractHname iscp.Hname, sigScheme *ed25519.KeyPair, nodeIndex ...int) *scclient.SCClient {
	return scclient.New(ch.Client(sigScheme, nodeIndex...), contractHname)
}

func (ch *Chain) CommitteeMultiClient() *multiclient.MultiClient {
	return multiclient.New(ch.CommitteeAPIHosts())
}

func (ch *Chain) DeployContract(name, progHashStr, description string, initParams map[string]interface{}) (*ledgerstate.Transaction, error) {
	programHash, err := hashing.HashValueFromBase58(progHashStr)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		root.ParamName:        name,
		root.ParamProgramHash: programHash,
		root.ParamDescription: description,
	}
	for k, v := range initParams {
		params[k] = v
	}
	tx, err := ch.OriginatorClient().Post1Request(
		root.Contract.Hname(),
		root.FuncDeployContract.Hname(),
		chainclient.PostRequestParams{
			Args: requestargs.New().AddEncodeSimpleMany(codec.MakeDict(params)),
		},
	)
	if err != nil {
		return nil, err
	}
	err = ch.CommitteeMultiClient().WaitUntilAllRequestsProcessed(ch.ChainID, tx, 30*time.Second)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (ch *Chain) DeployWasmContract(name, description string, progBinary []byte, initParams map[string]interface{}) (*ledgerstate.Transaction, hashing.HashValue, error) {
	blobFieldValues := codec.MakeDict(map[string]interface{}{
		blob.VarFieldVMType:             vmtypes.WasmTime,
		blob.VarFieldProgramBinary:      progBinary,
		blob.VarFieldProgramDescription: description,
	})

	quorum := (2*len(ch.CommitteeAPIHosts()))/3 + 1
	programHash, tx, err := ch.OriginatorClient().UploadBlob(blobFieldValues, ch.CommitteeAPIHosts(), quorum, 256)
	if err != nil {
		return nil, hashing.NilHash, err
	}
	err = ch.CommitteeMultiClient().WaitUntilAllRequestsProcessed(ch.ChainID, tx, 30*time.Second)
	if err != nil {
		return nil, hashing.NilHash, err
	}

	progBinaryBack, err := ch.GetBlobFieldValue(programHash, blob.VarFieldProgramBinary)
	if err != nil {
		return nil, hashing.NilHash, err
	}
	if !bytes.Equal(progBinary, progBinaryBack) {
		return nil, hashing.NilHash, fmt.Errorf("!bytes.Equal(progBinary, progBinaryBack)")
	}
	fmt.Printf("---- blob installed correctly len = %d\n", len(progBinaryBack))

	params := make(map[string]interface{})
	for k, v := range initParams {
		params[k] = v
	}
	params[root.ParamName] = name
	params[root.ParamProgramHash] = programHash
	params[root.ParamDescription] = description

	args := requestargs.New().AddEncodeSimpleMany(codec.MakeDict(params))
	tx, err = ch.OriginatorClient().Post1Request(
		root.Contract.Hname(),
		root.FuncDeployContract.Hname(),
		chainclient.PostRequestParams{
			Args: args,
		},
	)
	if err != nil {
		return nil, hashing.NilHash, err
	}
	err = ch.CommitteeMultiClient().WaitUntilAllRequestsProcessed(ch.ChainID, tx, 30*time.Second)
	if err != nil {
		return nil, hashing.NilHash, err
	}

	return tx, programHash, nil
}

func (ch *Chain) GetBlobFieldValue(blobHash hashing.HashValue, field string) ([]byte, error) {
	v, err := ch.Cluster.WaspClient(0).CallView(
		ch.ChainID, blob.Contract.Hname(), blob.FuncGetBlobField.Name,
		dict.Dict{
			blob.ParamHash:  blobHash[:],
			blob.ParamField: []byte(field),
		})
	if err != nil {
		return nil, err
	}
	if v.IsEmpty() {
		return nil, nil
	}
	ret, err := v.Get(blob.ParamBytes)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (ch *Chain) StartMessageCounter(expectations map[string]int) (*MessageCounter, error) {
	return NewMessageCounter(ch.Cluster, ch.CommitteeNodes, expectations)
}

func (ch *Chain) BlockIndex(nodeIndex ...int) (uint32, error) {
	cl := ch.SCClient(blocklog.Contract.Hname(), nil, nodeIndex...)
	ret, err := cl.CallView(blocklog.FuncGetLatestBlockInfo.Name, nil)
	if err != nil {
		return 0, err
	}
	n, _, err := codec.DecodeUint32(ret.MustGet(blocklog.ParamBlockIndex))
	return n, err
}

func (ch *Chain) GetAllBlockInfoRecordsReverse(nodeIndex ...int) ([]*blocklog.BlockInfo, error) {
	blockIndex, err := ch.BlockIndex(nodeIndex...)
	if err != nil {
		return nil, err
	}
	cl := ch.SCClient(blocklog.Contract.Hname(), nil, nodeIndex...)
	ret := make([]*blocklog.BlockInfo, 0, blockIndex+1)
	for idx := int(blockIndex); idx >= 0; idx-- {
		res, err := cl.CallView(blocklog.FuncGetBlockInfo.Name, dict.Dict{
			blocklog.ParamBlockIndex: codec.EncodeUint32(uint32(idx)),
		})
		if err != nil {
			return nil, err
		}
		bi, err := blocklog.BlockInfoFromBytes(uint32(idx), res.MustGet(blocklog.ParamBlockInfo))
		if err != nil {
			return nil, err
		}
		ret = append(ret, bi)
	}
	return ret, nil
}

func (ch *Chain) ContractRegistry(nodeIndex ...int) (map[iscp.Hname]*root.ContractRecord, error) {
	cl := ch.SCClient(root.Contract.Hname(), nil, nodeIndex...)
	ret, err := cl.CallView(root.FuncGetChainInfo.Name, nil)
	if err != nil {
		return nil, err
	}
	return root.DecodeContractRegistry(collections.NewMapReadOnly(ret, root.VarContractRegistry))
}

func (ch *Chain) GetCounterValue(inccounterSCHname iscp.Hname, nodeIndex ...int) (int64, error) {
	cl := ch.SCClient(inccounterSCHname, nil, nodeIndex...)
	ret, err := cl.CallView(inccounter.FuncGetCounter.Name, nil)
	if err != nil {
		return 0, err
	}
	n, _, err := codec.DecodeInt64(ret.MustGet(inccounter.VarCounter))
	return n, err
}

func (ch *Chain) GetStateVariable(contractHname iscp.Hname, key string, nodeIndex ...int) ([]byte, error) {
	cl := ch.SCClient(contractHname, nil, nodeIndex...)
	return cl.StateGet(key)
}

func (ch *Chain) GetRequestReceipt(reqID iscp.RequestID, nodeIndex ...int) (*blocklog.RequestReceipt, uint32, uint16, error) {
	cl := ch.SCClient(blocklog.Contract.Hname(), nil, nodeIndex...)
	ret, err := cl.CallView(blocklog.FuncGetRequestReceipt.Name, dict.Dict{blocklog.ParamRequestID: reqID.Bytes()})
	if err != nil {
		return nil, 0, 0, err
	}
	resultDecoder := kvdecoder.New(ret)
	binRec, err := resultDecoder.GetBytes(blocklog.ParamRequestRecord, nil)
	if err != nil || binRec == nil {
		return nil, 0, 0, err
	}
	rec, err := blocklog.RequestReceiptFromBytes(binRec)
	if err != nil {
		return nil, 0, 0, err
	}
	blockIndex := resultDecoder.MustGetUint32(blocklog.ParamBlockIndex)
	requestIndex := resultDecoder.MustGetUint16(blocklog.ParamRequestIndex)
	return rec, blockIndex, requestIndex, nil
}

func (ch *Chain) GetRequestReceiptsForBlock(blockIndex uint32, nodeIndex ...int) ([]*blocklog.RequestReceipt, error) {
	cl := ch.SCClient(blocklog.Contract.Hname(), nil, nodeIndex...)
	res, err := cl.CallView(blocklog.FuncGetRequestReceiptsForBlock.Name, dict.Dict{
		blocklog.ParamBlockIndex: codec.EncodeUint32(blockIndex),
	})
	if err != nil {
		return nil, err
	}
	recs := collections.NewArray16ReadOnly(res, blocklog.ParamRequestRecord)
	ret := make([]*blocklog.RequestReceipt, recs.MustLen())
	for i := range ret {
		data, err := recs.GetAt(uint16(i))
		if err != nil {
			return nil, err
		}
		ret[i], err = blocklog.RequestReceiptFromBytes(data)
		if err != nil {
			return nil, err
		}
		ret[i].WithBlockData(blockIndex, uint16(i))
	}
	return ret, nil
}
