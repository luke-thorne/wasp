package evmtypes

import (
	"bytes"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/rlp"
)

func EncodeStateDb(statedb *state.StateDB) []byte {
	var b bytes.Buffer
	err := rlp.Encode(&b, statedb)
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

func DecodeStateDb(b []byte) (*state.StateDB, error) {
	statedb := &state.StateDB{}
	err := rlp.DecodeBytes(b, statedb)
	return statedb, err
}