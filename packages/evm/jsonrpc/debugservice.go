package jsonrpc

import (
	"github.com/ethereum/go-ethereum/eth/tracers"
)

func NewDebugService(e tracers.Backend) *tracers.API {
	return tracers.NewAPI(e)
}
