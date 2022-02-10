package evmtypes

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

func EncodeHeader(header *types.Header) ([]byte) {
	b, err := rlp.EncodeToBytes(header)
	if err != nil {
		panic(err)
	}
	return b
}

func DecodeHeader(b []byte) (*types.Header, error) {
	header := &types.Header{}
	err := rlp.DecodeBytes(b, header)
	return header, err
}