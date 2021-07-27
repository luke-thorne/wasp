package blocklog

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/assert"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"golang.org/x/xerrors"
)

// SaveNextBlockInfo appends block info and returns its index
func SaveNextBlockInfo(partition kv.KVStore, blockInfo *BlockInfo) uint32 {
	registry := collections.NewArray32(partition, StateVarBlockRegistry)
	registry.MustPush(blockInfo.Bytes())
	ret := registry.MustLen() - 1
	return ret
}

// SaveControlAddressesIfNecessary saves new information about state address in the blocklog partition
// If state address does not change, it does nothing
func SaveControlAddressesIfNecessary(partition kv.KVStore, stateAddress, governingAddress ledgerstate.Address, blockIndex uint32) {
	registry := collections.NewArray32(partition, StateVarControlAddresses)
	l := registry.MustLen()
	if l != 0 {
		addrs, err := ControlAddressesFromBytes(registry.MustGetAt(l - 1))
		if err != nil {
			panic(fmt.Sprintf("SaveControlAddressesIfNecessary: %v", err))
		}
		if addrs.StateAddress.Equals(stateAddress) && addrs.GoverningAddress.Equals(governingAddress) {
			return
		}
	}
	rec := &ControlAddresses{
		StateAddress:     stateAddress,
		GoverningAddress: governingAddress,
		SinceBlockIndex:  blockIndex,
	}
	registry.MustPush(rec.Bytes())
}

// SaveRequestLogRecord appends request record to the record log and creates records for fast lookup
func SaveRequestLogRecord(partition kv.KVStore, rec *RequestReceipt, key RequestLookupKey) error {
	// save lookup record for fast lookup
	lookupTable := collections.NewMap(partition, StateVarRequestLookupIndex)
	digest := rec.RequestID.LookupDigest()
	var lst RequestLookupKeyList
	digestExists, err := lookupTable.HasAt(digest[:])
	if err != nil {
		return xerrors.Errorf("SaveRequestLogRecord: %w", err)
	}
	if !digestExists {
		// new digest, most common
		lst = make(RequestLookupKeyList, 0, 1)
	} else {
		// existing digest (should happen not often)
		bin, err := lookupTable.GetAt(digest[:])
		if err != nil {
			return xerrors.Errorf("SaveRequestLogRecord: %w", err)
		}
		if lst, err = RequestLookupKeyListFromBytes(bin); err != nil {
			return xerrors.Errorf("SaveRequestLogRecord: %w", err)
		}
	}
	for i := range lst {
		if lst[i] == key {
			// already in list. Not normal
			return xerrors.New("SaveRequestLogRecord: inconsistency: duplicate lookup key")
		}
	}
	lst = append(lst, key)
	if err := lookupTable.SetAt(digest[:], lst.Bytes()); err != nil {
		return xerrors.Errorf("SaveRequestLogRecord: %w", err)
	}
	// save the record. Key is a LookupKey
	if err = collections.NewMap(partition, StateVarRequestRecords).SetAt(key.Bytes(), rec.Bytes()); err != nil {
		return xerrors.Errorf("SaveRequestLogRecord: %w", err)
	}
	return nil
}

func SaveEvent(partition kv.KVStore, msg string, key EventLookupKey) error {
	if err := collections.NewMap(partition, StateVarRequestRecords).SetAt(key.Bytes(), []byte(msg)); err != nil {
		return xerrors.Errorf("SaveRequestLogRecord: %w", err)
	}
	return nil
}

func mustGetLookupKeyListFromReqID(partition kv.KVStoreReader, reqID *iscp.RequestID) RequestLookupKeyList {
	lookupTable := collections.NewMapReadOnly(partition, StateVarRequestLookupIndex)
	digest := reqID.LookupDigest()
	seen, err := lookupTable.HasAt(digest[:])
	if err != nil {
		return []RequestLookupKey{}
	}
	if !seen {
		return []RequestLookupKey{}
	}
	// the lookup record is here, have to check is it is not a collision of digests
	bin := lookupTable.MustGetAt(digest[:])
	lst, err := RequestLookupKeyListFromBytes(bin)
	if err != nil {
		panic("RequestKnown: data conversion error")
	}
	return lst
}

// RequestLookupKeyList contains multiple references for record entries with colliding digests, this function returns the correct record for the given requestID
func getCorrectRecordFromLookupKeyList(partition kv.KVStoreReader, keyList RequestLookupKeyList, reqID *iscp.RequestID) (*RequestLogRecord, error) {
	records := collections.NewMapReadOnly(partition, StateVarRequestRecords)
	for _, lookupKey := range keyList {
		recBytes, err := records.GetAt(lookupKey.Bytes())
		if err != nil {
			return nil, err
		}
		rec, err := RequestLogRecordFromBytes(recBytes)
		if err != nil {
			return nil, err
		}
		if rec.RequestID == *reqID {
			return rec, nil
		}
	}
	return nil, xerrors.Errorf("couldn't find record for requestID: %s", reqID.Base58())
}

// isRequestProcessedIntern does quick lookup to check if it wasn't seen yet
func isRequestProcessedIntern(partition kv.KVStoreReader, reqID *iscp.RequestID) (bool, error) {
	lst := mustGetLookupKeyListFromReqID(partition, reqID)
	record, err := getCorrectRecordFromLookupKeyList(partition, lst, reqID)
	return record != nil, err
}

func getRequestEventsIntern(partition kv.KVStoreReader, reqID *iscp.RequestID) ([]string, error) {
	lst := mustGetLookupKeyListFromReqID(partition, reqID)
	record, err := getCorrectRecordFromLookupKeyList(partition, lst, reqID)
	if err != nil {
		return nil, err
	}
	ret := []string{}
	eventIndex := uint16(0)
	events := collections.NewMapReadOnly(partition, StateVarRequestEvents)
	for {
		key := NewEventLookupKey(record.BlockIndex, record.RequestIndex, eventIndex)
		msg, err := events.GetAt(key.Bytes())
		if err != nil {
			return nil, err
		}
		if msg == nil {
			return ret, nil
		}
		ret = append(ret, string(msg))
		eventIndex++
	}
}

func getBlockEventsIntern(partition kv.KVStoreReader, blockIndex uint32) ([]string, error) {
	blockInfo, err := getRequestLogRecordsForBlock(partition, blockIndex)
	if err != nil {
		return nil, err
	}
	ret := []string{}
	events := collections.NewMapReadOnly(partition, StateVarRequestEvents)
	for reqIdx := uint16(0); reqIdx < blockInfo.NumOffLedgerRequests; reqIdx++ {
		eventIndex := uint16(0)
		for {
			key := NewEventLookupKey(blockIndex, reqIdx, eventIndex)
			msg, err := events.GetAt(key.Bytes())
			if err != nil {
				return nil, err
			}
			if msg == nil {
				break
			}
			ret = append(ret, string(msg))
			eventIndex++
		}
	}
	return ret, nil
}

func getRequestLogRecordsForBlock(partition kv.KVStoreReader, blockIndex uint32) (*BlockInfo, error) {
	if blockIndex == 0 {
		return nil, nil
	}
	blockInfoBin, found, err := getBlockInfoDataIntern(partition, blockIndex)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	blockInfo, err := BlockInfoFromBytes(blockIndex, blockInfoBin)
	if err != nil {
		return nil, err
	}
	return blockInfo, nil
}

func getRequestLogRecordsForBlockBin(partition kv.KVStoreReader, blockIndex uint32) ([][]byte, bool, error) {
	blockInfo, err := getRequestLogRecordsForBlock(partition, blockIndex)
	if err != nil || blockInfo == nil {
		return nil, false, err
	}
	ret := make([][]byte, blockInfo.TotalRequests)
	found := false
	for reqIdx := uint16(0); reqIdx < blockInfo.TotalRequests; reqIdx++ {
		ret[reqIdx], found = getRequestRecordDataByRef(partition, blockIndex, reqIdx)
		if !found {
			panic("getRequestLogRecordsForBlockBin: inconsistency: request record not found")
		}
	}
	return ret, true, nil
}

func getBlockInfoDataIntern(partition kv.KVStoreReader, blockIndex uint32) ([]byte, bool, error) {
	data, err := collections.NewArray32ReadOnly(partition, StateVarBlockRegistry).GetAt(blockIndex)
	return data, err == nil, err
}

func getRequestRecordDataByRef(partition kv.KVStoreReader, blockIndex uint32, requestIndex uint16) ([]byte, bool) {
	lookupKey := NewRequestLookupKey(blockIndex, requestIndex)
	lookupTable := collections.NewMapReadOnly(partition, StateVarRequestRecords)
	recBin := lookupTable.MustGetAt(lookupKey[:])
	if recBin == nil {
		return nil, false
	}
	return recBin, true
}

func getRequestRecordDataByRequestID(ctx iscp.SandboxView, reqID iscp.RequestID) ([]byte, uint32, uint16, bool) {
	lookupDigest := reqID.LookupDigest()
	lookupTable := collections.NewMapReadOnly(ctx.State(), StateVarRequestLookupIndex)
	lookupKeyListBin := lookupTable.MustGetAt(lookupDigest[:])
	if lookupKeyListBin == nil {
		return nil, 0, 0, false
	}
	a := assert.NewAssert(ctx.Log())
	lookupKeyList, err := RequestLookupKeyListFromBytes(lookupKeyListBin)
	a.RequireNoError(err)
	for i := range lookupKeyList {
		recBin, found := getRequestRecordDataByRef(ctx.State(), lookupKeyList[i].BlockIndex(), lookupKeyList[i].RequestIndex())
		a.Require(found, "inconsistency: request log record wasn't found by exact reference")
		rec, err := RequestReceiptFromBytes(recBin)
		a.RequireNoError(err)
		if rec.RequestID == reqID {
			return recBin, lookupKeyList[i].BlockIndex(), lookupKeyList[i].RequestIndex(), true
		}
	}
	return nil, 0, 0, false
}
