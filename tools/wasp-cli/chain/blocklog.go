package chain

import (
	"strconv"
	"time"

	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/vm/core/blocklog"
	"github.com/iotaledger/wasp/packages/vm/core/errors"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
	"github.com/spf13/cobra"
)

func blockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "block [index]",
		Short: "Get information about a block given its index, or latest block if missing",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			bi := fetchBlockInfo(args)
			log.Printf("Block index: %d\n", bi.BlockIndex)
			log.Printf("Timestamp: %s\n", bi.Timestamp.UTC().Format(time.RFC3339))
			log.Printf("Total requests: %d\n", bi.TotalRequests)
			log.Printf("Successful requests: %d\n", bi.NumSuccessfulRequests)
			log.Printf("Off-ledger requests: %d\n", bi.NumOffLedgerRequests)
			log.Printf("\n")
			logRequestsInBlock(bi.BlockIndex)
			log.Printf("\n")
			logEventsInBlock(bi.BlockIndex)
		},
	}
}

func fetchBlockInfo(args []string) *blocklog.BlockInfo {
	if len(args) == 0 {
		ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetBlockInfo.Name, nil)
		log.Check(err)
		index, err := codec.DecodeUint32(ret.MustGet(blocklog.ParamBlockIndex))
		log.Check(err)
		b, err := blocklog.BlockInfoFromBytes(index, ret.MustGet(blocklog.ParamBlockInfo))
		log.Check(err)
		return b
	}
	index, err := strconv.Atoi(args[0])
	log.Check(err)
	ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetBlockInfo.Name, dict.Dict{
		blocklog.ParamBlockIndex: codec.EncodeUint32(uint32(index)),
	})
	log.Check(err)
	b, err := blocklog.BlockInfoFromBytes(uint32(index), ret.MustGet(blocklog.ParamBlockInfo))
	log.Check(err)
	return b
}

func logRequestsInBlock(index uint32) {
	ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetRequestReceiptsForBlock.Name, dict.Dict{
		blocklog.ParamBlockIndex: codec.EncodeUint32(index),
	})
	log.Check(err)
	arr := collections.NewArray16ReadOnly(ret, blocklog.ParamRequestRecord)
	for i := uint16(0); i < arr.MustLen(); i++ {
		receipt, err := blocklog.RequestReceiptFromBytes(arr.MustGetAt(i))
		log.Check(err)
		logReceipt(receipt, i)
	}
}

func logReceipt(receipt *blocklog.RequestReceipt, index ...uint16) {
	req := receipt.Request

	kind := "on-ledger"
	if req.IsOffLedger() {
		kind = "off-ledger"
	}

	args := req.Params()
	var argsTree interface{} = "(empty)"
	if len(args) > 0 {
		argsTree = args
	}

	errMsg := "(empty)"
	if receipt.Error != nil {
		resolved, err := errors.Resolve(receipt.Error, func(contractName string, funcName string, params dict.Dict) (dict.Dict, error) {
			return SCClient(iscp.Hn(contractName)).CallView(funcName, params)
		})
		log.Check(err)
		errMsg = resolved.Error()
	}

	tree := []log.TreeItem{
		{K: "Kind", V: kind},
		{K: "Sender", V: req.SenderAccount().String()},
		{K: "Contract Hname", V: req.CallTarget().Contract.String()},
		{K: "Entry point", V: req.CallTarget().EntryPoint.String()},
		{K: "Arguments", V: argsTree},
		{K: "Error", V: errMsg},
	}
	if len(index) > 0 {
		log.Printf("Request #%d (%s):\n", index[0], req.ID().String())
	} else {
		log.Printf("Request %s:\n", req.ID().String())
	}
	log.PrintTree(tree, 2, 2)
}

func logEventsInBlock(index uint32) {
	ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetEventsForBlock.Name, dict.Dict{
		blocklog.ParamBlockIndex: codec.EncodeUint32(index),
	})
	log.Check(err)
	logEvents(ret)
}

func requestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "request <request-id>",
		Short: "Get information about a request given its ID",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reqID, err := iscp.RequestIDFromString(args[0])
			log.Check(err)
			ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetRequestReceipt.Name, dict.Dict{
				blocklog.ParamRequestID: codec.EncodeRequestID(reqID),
			})
			log.Check(err)

			blockIndex, err := codec.DecodeUint32(ret.MustGet(blocklog.ParamBlockIndex))
			log.Check(err)
			receipt, err := blocklog.RequestReceiptFromBytes(ret.MustGet(blocklog.ParamRequestRecord))
			log.Check(err)

			log.Printf("Request found in block %d\n\n", blockIndex)
			logReceipt(receipt)
			log.Printf("\n")
			logEventsInRequest(reqID)
			log.Printf("\n")
		},
	}
}

func logEventsInRequest(reqID iscp.RequestID) {
	ret, err := SCClient(blocklog.Contract.Hname()).CallView(blocklog.ViewGetEventsForRequest.Name, dict.Dict{
		blocklog.ParamRequestID: codec.EncodeRequestID(reqID),
	})
	log.Check(err)
	logEvents(ret)
}

func logEvents(ret dict.Dict) {
	arr := collections.NewArray16ReadOnly(ret, blocklog.ParamEvent)
	header := []string{"event"}
	rows := make([][]string, arr.MustLen())
	for i := uint16(0); i < arr.MustLen(); i++ {
		rows[i] = []string{string(arr.MustGetAt(i))}
	}
	log.Printf("Total %d events\n", arr.MustLen())
	log.PrintTable(header, rows)
}
