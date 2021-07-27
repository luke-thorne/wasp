// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package messages

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/iscp/request"
	"golang.org/x/xerrors"
)

type OffLedgerRequestMsg struct {
	ChainID *iscp.ChainID
	Req     *request.OffLedger
}

func NewOffLedgerRequestMsg(chainID *iscp.ChainID, req *request.OffLedger) *OffLedgerRequestMsg {
	return &OffLedgerRequestMsg{
		ChainID: chainID,
		Req:     req,
	}
}

func (msg *OffLedgerRequestMsg) Bytes() []byte {
	return marshalutil.New().
		Write(msg.ChainID).
		Write(msg.Req).
		Bytes()
}

func OffLedgerRequestMsgFromBytes(data []byte) (*OffLedgerRequestMsg, error) {
	mu := marshalutil.New(data)
	chainID, err := iscp.ChainIDFromMarshalUtil(mu)
	if err != nil {
		return nil, err
	}
	req, err := request.FromMarshalUtil(mu)
	if err != nil {
		return nil, err
	}
	reqCasted, ok := req.(*request.OffLedger)
	if !ok {
		return nil, xerrors.New("OffLedgerRequestMsgFromBytes: wrong type of request data")
	}
	return &OffLedgerRequestMsg{
		ChainID: chainID,
		Req:     reqCasted,
	}, nil
}
