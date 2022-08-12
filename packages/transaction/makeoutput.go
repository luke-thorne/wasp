package transaction

import (
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/parameters"
)

// BasicOutputFromPostData creates extended output object from parameters.
// It automatically adjusts amount of base tokens required for the storage deposit
func BasicOutputFromPostData(
	senderAddress iotago.Address,
	senderContract isc.Hname,
	par isc.RequestParameters,
) *iotago.BasicOutput {
	metadata := par.Metadata
	if metadata == nil {
		// if metadata is not specified, target is nil. It corresponds to sending funds to the plain L1 address
		metadata = &isc.SendMetadata{}
	}

	ret := MakeBasicOutput(
		par.TargetAddress,
		senderAddress,
		par.FungibleTokens,
		&isc.RequestMetadata{
			SenderContract: senderContract,
			TargetContract: metadata.TargetContract,
			EntryPoint:     metadata.EntryPoint,
			Params:         metadata.Params,
			Allowance:      metadata.Allowance,
			GasBudget:      metadata.GasBudget,
		},
		par.Options,
		!par.AdjustToMinimumStorageDeposit,
	)
	return ret
}

// MakeBasicOutput creates new output from input parameters.
// Auto adjusts minimal storage deposit if the notAutoAdjust flag is absent or false
// If auto adjustment to storage deposit is disabled and not enough base tokens, returns an error
func MakeBasicOutput(
	targetAddress iotago.Address,
	senderAddress iotago.Address,
	assets *isc.FungibleTokens,
	metadata *isc.RequestMetadata,
	options isc.SendOptions,
	disableAutoAdjustStorageDeposit ...bool,
) *iotago.BasicOutput {
	if assets == nil {
		assets = &isc.FungibleTokens{}
	}
	out := &iotago.BasicOutput{
		Amount:       assets.BaseTokens,
		NativeTokens: assets.Tokens,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: targetAddress},
		},
	}
	if senderAddress != nil {
		out.Features = append(out.Features, &iotago.SenderFeature{
			Address: senderAddress,
		})
	}
	if metadata != nil {
		out.Features = append(out.Features, &iotago.MetadataFeature{
			Data: metadata.Bytes(),
		})
	}
	if !options.Timelock.IsZero() {
		cond := &iotago.TimelockUnlockCondition{
			UnixTime: uint32(options.Timelock.Unix()),
		}
		out.Conditions = append(out.Conditions, cond)
	}
	if options.Expiration != nil {
		cond := &iotago.ExpirationUnlockCondition{
			ReturnAddress: options.Expiration.ReturnAddress,
		}
		if !options.Expiration.Time.IsZero() {
			cond.UnixTime = uint32(options.Expiration.Time.Unix())
		}
		out.Conditions = append(out.Conditions, cond)
	}

	// Adjust to minimum storage deposit, if needed
	if len(disableAutoAdjustStorageDeposit) > 0 && disableAutoAdjustStorageDeposit[0] {
		return out
	}

	storageDeposit := parameters.L1().Protocol.RentStructure.MinRent(out)
	if out.Deposit() < storageDeposit {
		// adjust the amount to the minimum required
		out.Amount = storageDeposit
	}

	return out
}

func NFTOutputFromPostData(
	senderAddress iotago.Address,
	senderContract isc.Hname,
	par isc.RequestParameters,
	nft *isc.NFT,
) *iotago.NFTOutput {
	basicOutput := BasicOutputFromPostData(senderAddress, senderContract, par)
	out := NftOutputFromBasicOutput(basicOutput, nft)

	if !par.AdjustToMinimumStorageDeposit {
		return out
	}
	storageDeposit := parameters.L1().Protocol.RentStructure.MinRent(out)
	if out.Deposit() < storageDeposit {
		// adjust the amount to the minimum required
		out.Amount = storageDeposit
	}
	return out
}

func NftOutputFromBasicOutput(o *iotago.BasicOutput, nft *isc.NFT) *iotago.NFTOutput {
	return &iotago.NFTOutput{
		Amount:       o.Amount,
		NativeTokens: o.NativeTokens,
		Features:     o.Features,
		Conditions:   o.Conditions,
		NFTID:        nft.ID,
		ImmutableFeatures: iotago.Features{
			&iotago.IssuerFeature{Address: nft.Issuer},
			&iotago.MetadataFeature{Data: nft.Metadata},
		},
	}
}

func AssetsFromOutput(o iotago.Output) *isc.FungibleTokens {
	return &isc.FungibleTokens{
		BaseTokens: o.Deposit(),
		Tokens:     o.NativeTokenList(),
	}
}
