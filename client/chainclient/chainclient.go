package chainclient

import (
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/client"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/iscp"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/nodeconn"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
	"github.com/iotaledger/wasp/packages/vm/gas"
)

// Client allows to interact with a specific chain in the node, for example to send on-ledger or off-ledger requests
type Client struct {
	Layer1Client nodeconn.L1Client
	WaspClient   *client.WaspClient
	ChainID      *iscp.ChainID
	KeyPair      *cryptolib.KeyPair
	nonces       map[string]uint64
}

// New creates a new chainclient.Client
func New(
	layer1Client nodeconn.L1Client,
	waspClient *client.WaspClient,
	chainID *iscp.ChainID,
	keyPair *cryptolib.KeyPair,
) *Client {
	return &Client{
		Layer1Client: layer1Client,
		WaspClient:   waspClient,
		ChainID:      chainID,
		KeyPair:      keyPair,
		nonces:       make(map[string]uint64),
	}
}

type PostRequestParams struct {
	Transfer  *iscp.FungibleTokens
	Args      dict.Dict
	Nonce     uint64
	NFT       *iscp.NFT
	Allowance *iscp.Allowance
	GasBudget *uint64
}

func defaultParams(params ...PostRequestParams) PostRequestParams {
	if len(params) > 0 {
		return params[0]
	}
	return PostRequestParams{}
}

// Post1Request sends an on-ledger transaction with one request on it to the chain
func (c *Client) Post1Request(
	contractHname iscp.Hname,
	entryPoint iscp.Hname,
	params ...PostRequestParams,
) (*iotago.Transaction, error) {
	outputsSet, err := c.Layer1Client.OutputMap(c.KeyPair.Address())
	if err != nil {
		return nil, err
	}
	return c.post1RequestWithOutputs(contractHname, entryPoint, outputsSet, params...)
}

// PostNRequest sends n consecutive on-ledger transactions with one request on each, to the chain
func (c *Client) PostNRequests(
	contractHname iscp.Hname,
	entryPoint iscp.Hname,
	requestsCount int,
	params ...PostRequestParams,
) ([]*iotago.Transaction, error) {
	var err error
	outputs, err := c.Layer1Client.OutputMap(c.KeyPair.Address())
	if err != nil {
		return nil, err
	}
	transactions := make([]*iotago.Transaction, requestsCount)
	for i := 0; i < requestsCount; i++ {
		transactions[i], err = c.post1RequestWithOutputs(contractHname, entryPoint, outputs, params...)
		if err != nil {
			return nil, err
		}
		txID, err := transactions[i].ID()
		if err != nil {
			return nil, err
		}
		for _, input := range transactions[i].Essence.Inputs {
			if utxoInput, ok := input.(*iotago.UTXOInput); ok {
				delete(outputs, utxoInput.ID())
			}
		}
		for index, output := range transactions[i].Essence.Outputs {
			if basicOutput, ok := output.(*iotago.BasicOutput); ok {
				if basicOutput.Ident().Equal(c.KeyPair.Address()) {
					outputID := iotago.OutputIDFromTransactionIDAndIndex(txID, uint16(index))
					outputs[outputID] = transactions[i].Essence.Outputs[index]
				}
			}
		}
	}
	return transactions, nil
}

func (c *Client) post1RequestWithOutputs(
	contractHname iscp.Hname,
	entryPoint iscp.Hname,
	outputs iotago.OutputSet,
	params ...PostRequestParams,
) (*iotago.Transaction, error) {
	par := defaultParams(params...)
	var gasBudget uint64
	if par.GasBudget == nil {
		gasBudget = gas.MaxGasPerCall
	} else {
		gasBudget = *par.GasBudget
	}
	tx, err := transaction.NewRequestTransaction(
		transaction.NewRequestTransactionParams{
			SenderKeyPair:    c.KeyPair,
			SenderAddress:    c.KeyPair.Address(),
			UnspentOutputs:   outputs,
			UnspentOutputIDs: iscp.OutputSetToOutputIDs(outputs),
			Request: &iscp.RequestParameters{
				TargetAddress:              c.ChainID.AsAddress(),
				FungibleTokens:             par.Transfer,
				AdjustToMinimumDustDeposit: false,
				Metadata: &iscp.SendMetadata{
					TargetContract: contractHname,
					EntryPoint:     entryPoint,
					Params:         par.Args,
					Allowance:      par.Allowance,
					GasBudget:      gasBudget,
				},
			},
			NFT: par.NFT,
		},
	)
	if err != nil {
		return nil, err
	}
	err = c.Layer1Client.PostTx((tx))
	return tx, err
}

// PostOffLedgerRequest sends an off-ledger tx via the wasp node web api
func (c *Client) PostOffLedgerRequest(
	contractHname iscp.Hname,
	entrypoint iscp.Hname,
	params ...PostRequestParams,
) (iscp.OffLedgerRequest, error) {
	par := defaultParams(params...)
	if par.Nonce == 0 {
		c.nonces[c.KeyPair.Address().Key()]++
		par.Nonce = c.nonces[c.KeyPair.Address().Key()]
	}
	var gasBudget uint64
	if par.GasBudget == nil {
		gasBudget = gas.MaxGasPerCall
	} else {
		gasBudget = *par.GasBudget
	}
	req := iscp.NewOffLedgerRequest(c.ChainID, contractHname, entrypoint, par.Args, par.Nonce)
	req.WithAllowance(par.Allowance)
	if par.GasBudget != nil {
		req = req.WithGasBudget(gasBudget)
	}
	req.WithNonce(par.Nonce)
	signed := req.Sign(c.KeyPair)
	return signed, c.WaspClient.PostOffLedgerRequest(c.ChainID, signed)
}

func (c *Client) DepositFunds(n uint64) (*iotago.Transaction, error) {
	return c.Post1Request(accounts.Contract.Hname(), accounts.FuncDeposit.Hname(), PostRequestParams{
		Transfer: iscp.NewFungibleTokens(n, nil),
	})
}

// NewPostRequestParams simplifies encoding of request parameters
func NewPostRequestParams(p ...interface{}) *PostRequestParams {
	return &PostRequestParams{
		Args:     parseParams(p),
		Transfer: iscp.NewFungibleTokens(0, nil),
	}
}

func (par *PostRequestParams) WithTransfer(transfer *iscp.FungibleTokens) *PostRequestParams {
	par.Transfer = transfer
	return par
}

func (par *PostRequestParams) WithIotas(i uint64) *PostRequestParams {
	par.Transfer.AddIotas(i)
	return par
}

func (par *PostRequestParams) WithGasBudget(budget uint64) *PostRequestParams {
	par.GasBudget = &budget
	return par
}

func parseParams(params []interface{}) dict.Dict {
	if len(params) == 1 {
		return params[0].(dict.Dict)
	}
	return codec.MakeDict(toMap(params))
}

// makes map without hashing
func toMap(params []interface{}) map[string]interface{} {
	par := make(map[string]interface{})
	if len(params) == 0 {
		return par
	}
	if len(params)%2 != 0 {
		panic("toMap: len(params) % 2 != 0")
	}
	for i := 0; i < len(params)/2; i++ {
		key, ok := params[2*i].(string)
		if !ok {
			panic("toMap: string expected")
		}
		par[key] = params[2*i+1]
	}
	return par
}
