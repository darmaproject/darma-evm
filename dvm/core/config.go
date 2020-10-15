package dvm

import (
	"github.com/darmaproject/darmasuite/dvm/core/vm"
	"github.com/darmaproject/darmasuite/dvm/core/wavm"
	"github.com/darmaproject/darmasuite/dvm/params"
)

func GetVMConfig() vm.Config {
	//return vm.Config{}
	return vm.Config{Debug: true, Tracer: wavm.NewWasmLogger(&vm.LogConfig{Debug: true})}
}

func GetChainCOnfig() *params.ChainConfig {
	return &params.ChainConfig{}
}
