package evmtypes

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/core/state"
)

func EncodeStateDb(statedb *state.StateDB) []byte {
	b, err := json.Marshal(*statedb)
	if err != nil {
		panic(err)
	}
	return b
}

func DecodeStateDb(b []byte) (*state.StateDB, error) {
	res := state.StateDB{}
	err := json.Unmarshal(b, &res)
	return &res, err
}