// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package wasmhost

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go"
)

type WasmTimeVM struct {
	WasmVMBase
	instance  *wasmtime.Instance
	interrupt *wasmtime.InterruptHandle
	linker    *wasmtime.Linker
	memory    *wasmtime.Memory
	module    *wasmtime.Module
	store     *wasmtime.Store
}

func NewWasmTimeVM() WasmVM {
	vm := &WasmTimeVM{}
	config := wasmtime.NewConfig()
	config.SetInterruptable(true)
	vm.store = wasmtime.NewStore(wasmtime.NewEngineWithConfig(config))
	vm.interrupt, _ = vm.store.InterruptHandle()
	return vm
}

func (vm *WasmTimeVM) NewInstance() WasmVM {
	return &WasmTimeVM{store: vm.store, module: vm.module, interrupt: vm.interrupt}
}

func (vm *WasmTimeVM) Interrupt() {
	vm.interrupt.Interrupt()
}

func (vm *WasmTimeVM) LinkHost(impl WasmVM, host *WasmHost) error {
	vm.linker = wasmtime.NewLinker(vm.store)
	_ = vm.WasmVMBase.LinkHost(impl, host)

	err := vm.linker.DefineFunc("WasmLib", "hostGetBytes", vm.HostGetBytes)
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostGetKeyID", vm.HostGetKeyID)
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostGetObjectID", vm.HostGetObjectID)
	if err != nil {
		return err
	}
	err = vm.linker.DefineFunc("WasmLib", "hostSetBytes", vm.HostSetBytes)
	if err != nil {
		return err
	}

	// AssemblyScript Wasm versions uses this one to write panic message to console
	err = vm.linker.DefineFunc("env", "abort", vm.EnvAbort)
	if err != nil {
		return err
	}

	// TinyGo Wasm versions uses this one to write panic message to console
	err = vm.linker.DefineFunc("wasi_unstable", "fd_write", vm.HostFdWrite)
	if err != nil {
		return err
	}
	return vm.linker.DefineFunc("wasi_snapshot_preview1", "fd_write", vm.HostFdWrite)
}

func (vm *WasmTimeVM) LoadWasm(wasmData []byte) (err error) {
	vm.module, err = wasmtime.NewModule(vm.store.Engine, wasmData)
	if err != nil {
		return err
	}
	return vm.Instantiate()
}

func (vm *WasmTimeVM) Instantiate() (err error) {
	vm.instance, err = vm.linker.Instantiate(vm.module)
	if err != nil {
		return err
	}
	memory := vm.instance.GetExport("memory")
	if memory == nil {
		return errors.New("no memory export")
	}
	vm.memory = memory.Memory()
	if vm.memory == nil {
		return errors.New("not a memory type")
	}
	return nil
}

func (vm *WasmTimeVM) PoolSize() int {
	return 10
}

func (vm *WasmTimeVM) RunFunction(functionName string, args ...interface{}) error {
	export := vm.instance.GetExport(functionName)
	if export == nil {
		return errors.New("unknown export function: '" + functionName + "'")
	}
	return vm.Run(func() (err error) {
		_, err = export.Func().Call(args...)
		return err
	})
}

func (vm *WasmTimeVM) RunScFunction(index int32) error {
	export := vm.instance.GetExport("on_call")
	if export == nil {
		return errors.New("unknown export function: 'on_call'")
	}

	frame := vm.PreCall()
	defer vm.PostCall(frame)

	return vm.Run(func() (err error) {
		_, err = export.Func().Call(index)
		return err
	})
}

func (vm *WasmTimeVM) UnsafeMemory() []byte {
	return vm.memory.UnsafeData()
}
