package codec

import (
	"fmt"
	"time"

	"github.com/iotaledger/wasp/packages/iscp/colored"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/wasp/packages/hashing"
	"github.com/iotaledger/wasp/packages/iscp"
)

func Encode(v interface{}) []byte {
	switch vt := v.(type) {
	case int: // default to int64
		return EncodeInt64(int64(vt))
	case byte:
		return EncodeInt64(int64(vt))
	case int16:
		return EncodeInt16(vt)
	case int32:
		return EncodeInt32(vt)
	case int64:
		return EncodeInt64(vt)
	case uint16:
		return EncodeUint16(vt)
	case uint32:
		return EncodeUint32(vt)
	case uint64:
		return EncodeUint64(vt)
	case string:
		return EncodeString(vt)
	case []byte:
		return vt
	case *hashing.HashValue:
		return EncodeHashValue(*vt)
	case hashing.HashValue:
		return EncodeHashValue(vt)
	case ledgerstate.Address:
		return EncodeAddress(vt)
	case *colored.Color:
		return EncodeColor(*vt)
	case colored.Color:
		return EncodeColor(vt)
	case *iscp.ChainID:
		return EncodeChainID(*vt)
	case iscp.ChainID:
		return EncodeChainID(vt)
	case *iscp.AgentID:
		return EncodeAgentID(vt)
	case iscp.AgentID:
		return EncodeAgentID(&vt)
	case iscp.RequestID:
		return EncodeRequestID(vt)
	case *iscp.RequestID:
		return EncodeRequestID(*vt)
	case iscp.Hname:
		return vt.Bytes()
	case time.Time:
		return EncodeTime(vt)

	default:
		panic(fmt.Sprintf("Can't encode value %v", v))
	}
}
