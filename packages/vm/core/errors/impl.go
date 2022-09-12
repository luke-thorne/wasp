package errors

import (
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
)

var Processor = Contract.Processor(initialize,
	FuncRegisterError.WithHandler(funcRegisterError),
	ViewGetErrorMessageFormat.WithHandler(funcGetErrorMessageFormat),
)

func initialize(ctx isc.Sandbox) dict.Dict {
	// storing hname as a terminal value of the contract's state root.
	// This way we will be able to retrieve commitment to the contract's state
	ctx.State().Set("", ctx.Contract().Bytes())

	ctx.Log().Debugf("errors.initialize.success hname = %s", Contract.Hname().String())
	return nil
}

func funcRegisterError(ctx isc.Sandbox) dict.Dict {
	ctx.Log().Debugf("Registering error")
	e := NewStateErrorCollectionWriter(ctx.State(), ctx.Contract())

	params := kvdecoder.New(ctx.Params())
	errorMessageFormat := params.MustGetString(ParamErrorMessageFormat)

	if errorMessageFormat == "" {
		panic(coreerrors.ErrMessageFormatEmpty)
	}

	template, err := e.Register(errorMessageFormat)
	ctx.RequireNoError(err)

	return dict.Dict{ParamErrorCode: codec.EncodeVMErrorCode(template.Code())}
}

func funcGetErrorMessageFormat(ctx isc.SandboxView) dict.Dict {
	code := codec.MustDecodeVMErrorCode(ctx.Params().MustGet(ParamErrorCode))

	template, err := getErrorMessageFormat(ctx.StateR(), code)
	ctx.RequireNoError(err)

	return dict.Dict{ParamErrorMessageFormat: codec.EncodeString(template.MessageFormat())}
}

func getErrorMessageFormat(state kv.KVStoreReader, code isc.VMErrorCode) (*isc.VMErrorTemplate, error) {
	var e coreerrors.ErrorCollection
	if code.ContractID == isc.VMCoreErrorContractID {
		e = coreerrors.All()
	} else {
		e = NewStateErrorCollectionReader(state, code.ContractID)
	}
	return e.Get(code.ID)
}
