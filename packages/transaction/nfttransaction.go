package transaction

import (
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/parameters"
)

type MintNFTTransactionParams struct {
	IssuerKeyPair     *cryptolib.KeyPair
	Target            iotago.Address
	UnspentOutputs    iotago.OutputSet
	UnspentOutputIDs  iotago.OutputIDs
	ImmutableMetadata []byte
}

func NewMintNFTTransaction(par MintNFTTransactionParams) (*iotago.Transaction, error) {
	issuerAddress := par.IssuerKeyPair.Address()

	out := &iotago.NFTOutput{
		NFTID: iotago.NFTID{},
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: par.Target},
		},
		ImmutableFeatures: iotago.Features{
			&iotago.IssuerFeature{Address: issuerAddress},
			&iotago.MetadataFeature{Data: par.ImmutableMetadata},
		},
	}
	storageDeposit := parameters.L1().Protocol.RentStructure.MinRent(out)
	out.Amount = storageDeposit

	outputs := iotago.Outputs{out}

	inputIDs, remainder, err := computeInputsAndRemainder(issuerAddress, storageDeposit, nil, nil, par.UnspentOutputs, par.UnspentOutputIDs)
	if err != nil {
		return nil, err
	}
	if remainder != nil {
		outputs = append(outputs, remainder)
	}

	inputsCommitment := inputIDs.OrderedSet(par.UnspentOutputs).MustCommitment()
	return CreateAndSignTx(inputIDs, inputsCommitment, outputs, par.IssuerKeyPair, parameters.L1().Protocol.NetworkID())
}
