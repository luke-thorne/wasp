package jsonrpc

import (
	"github.com/ethereum/go-ethereum/eth/tracers"
)

type TracerAPI struct{
	 *tracers.API
}

func NewDebugService(e tracers.Backend) *TracerAPI {
	return &TracerAPI{tracers.NewAPI(e)}
}
