package chainclient

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/wasp/client"
	"github.com/iotaledger/wasp/client/goshimmer"
	"github.com/iotaledger/wasp/packages/coretypes"
	"github.com/iotaledger/wasp/packages/coretypes/chainid"
	"github.com/iotaledger/wasp/packages/coretypes/request"
	"github.com/iotaledger/wasp/packages/coretypes/requestargs"
	"github.com/iotaledger/wasp/packages/transaction"
)

// Client allows to interact with a specific chain in the node, for example to send on-ledger or off-ledger requests
type Client struct {
	GoshimmerClient *goshimmer.Client
	WaspClient      *client.WaspClient
	ChainID         chainid.ChainID
	KeyPair         *ed25519.KeyPair
}

// New creates a new chainclient.Client
func New(
	goshimmerClient *goshimmer.Client,
	waspClient *client.WaspClient,
	chainID chainid.ChainID,
	keyPair *ed25519.KeyPair,
) *Client {
	return &Client{
		GoshimmerClient: goshimmerClient,
		WaspClient:      waspClient,
		ChainID:         chainID,
		KeyPair:         keyPair,
	}
}

type PostRequestParams struct {
	Transfer *ledgerstate.ColoredBalances
	Args     requestargs.RequestArgs
}

// Post1Request sends an on-ledger transaction with one request on it to the chain
func (c *Client) Post1Request(
	contractHname coretypes.Hname,
	entryPoint coretypes.Hname,
	params ...PostRequestParams,
) (*ledgerstate.Transaction, error) {
	par := PostRequestParams{}
	if len(params) > 0 {
		par = params[0]
	}
	return c.GoshimmerClient.PostRequestTransaction(transaction.NewRequestTransactionParams{
		SenderKeyPair: c.KeyPair,
		Requests: []transaction.RequestParams{{
			ChainID:    c.ChainID,
			Contract:   contractHname,
			EntryPoint: entryPoint,
			Transfer:   par.Transfer,
			Args:       par.Args,
		}},
	})
}

// PostOffLedgerRequest sends an off-ledger tx via the wasp node web api
func (c *Client) PostOffLedgerRequest(
	contractHname coretypes.Hname,
	entrypoint coretypes.Hname,
	params ...PostRequestParams,
) (*request.RequestOffLedger, error) {
	par := PostRequestParams{}
	if len(params) > 0 {
		par = params[0]
	}
	offledgerReq := request.NewRequestOffLedger(contractHname, entrypoint, par.Args).WithTransfer(par.Transfer)
	offledgerReq.Sign(c.KeyPair)
	return offledgerReq, c.WaspClient.PostOffLedgerRequest(&c.ChainID, offledgerReq)
}
