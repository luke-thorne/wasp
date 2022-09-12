package chain

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/client/chainclient"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/transaction"
	"github.com/iotaledger/wasp/packages/vm/gas"
	"github.com/iotaledger/wasp/tools/wasp-cli/util"
	"github.com/spf13/cobra"
)

func postRequest(hname, fname string, params chainclient.PostRequestParams, offLedger, adjustStorageDeposit bool) {
	scClient := SCClient(isc.Hn(hname))

	if offLedger {
		params.Nonce = uint64(time.Now().UnixNano())
		util.WithOffLedgerRequest(GetCurrentChainID(), func() (isc.OffLedgerRequest, error) {
			return scClient.PostOffLedgerRequest(fname, params)
		})
		return
	}

	if !adjustStorageDeposit {
		// check if there are enough funds for SD
		output := transaction.MakeBasicOutput(
			GetCurrentChainID().AsAddress(),
			scClient.ChainClient.KeyPair.Address(),
			params.Transfer,
			&isc.RequestMetadata{
				SenderContract: 0,
				TargetContract: isc.Hn(hname),
				EntryPoint:     isc.Hn(fname),
				Params:         params.Args,
				Allowance:      params.Allowance,
				GasBudget:      gas.MaxGasPerCall,
			},
			isc.SendOptions{},
			true,
		)
		util.SDAdjustmentPrompt(output)
	}

	util.WithSCTransaction(GetCurrentChainID(), func() (*iotago.Transaction, error) {
		return scClient.PostRequest(fname, params)
	})
}

func postRequestCmd() *cobra.Command {
	var transfer []string
	var allowance []string
	var offLedger bool
	var adjustStorageDeposit bool

	cmd := &cobra.Command{
		Use:   "post-request <name> <funcname> [params]",
		Short: "Post a request to a contract",
		Long:  "Post a request to contract <name>, function <funcname> with given params.",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			hname := args[0]
			fname := args[1]

			allowanceTokens := util.ParseFungibleTokens(allowance)
			params := chainclient.PostRequestParams{
				Args:      util.EncodeParams(args[2:]),
				Transfer:  util.ParseFungibleTokens(transfer),
				Allowance: isc.NewAllowanceFungibleTokens(allowanceTokens),
			}
			postRequest(hname, fname, params, offLedger, adjustStorageDeposit)
		},
	}

	cmd.Flags().StringSliceVarP(&allowance, "allowance", "l", []string{},
		"include allowance as part of the transaction. Format: <token-id>:<amount>,<token-id>:amount...")

	cmd.Flags().StringSliceVarP(&transfer, "transfer", "t", []string{},
		"include a funds transfer as part of the transaction. Format: <token-id>:<amount>,<token-id>:amount...",
	)
	cmd.Flags().BoolVarP(&offLedger, "off-ledger", "o", false,
		"post an off-ledger request",
	)
	cmd.Flags().BoolVarP(&adjustStorageDeposit, "adjust-storage-deposit", "s", false, "adjusts the amount of base tokens sent, if it's lower than the min storage deposit required")

	return cmd
}
