package dvm

import (
	"errors"
	"github.com/darmaproject/darmasuite/dvm/common"
	"github.com/darmaproject/darmasuite/dvm/core/vm"
	"github.com/darmaproject/darmasuite/dvm/core/vm/interface"
	"github.com/darmaproject/darmasuite/dvm/params"
	"github.com/romana/rlog"
	"math"
	"math/big"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      inter.StateDB
	vm         vm.VM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte

	ToIsEmpty() bool
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/params.TxDataNonZeroGas < nz {
			rlog.Error((math.MaxUint64-gas)/params.TxDataNonZeroGas, nz)
			return 0, vm.ErrOutOfGas
		}
		gas += nz * params.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			rlog.Error((math.MaxUint64-gas)/params.TxDataZeroGas, z)
			return 0, vm.ErrOutOfGas
		}
		gas += z * params.TxDataZeroGas
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(vm vm.VM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		vm:       vm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    vm.GetStateDb(),
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any VM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(vm vm.VM, msg Message, gp *GasPool) ([]byte, uint64, []byte, error) {
<<<<<<< HEAD
	rlog.Info("---ApplyMessage---")
=======
	rlog.Error("---ApplyMessage---")
>>>>>>> 4c17f25eda6f8eefa5bdc69a367db53ccfd879fc
	return NewStateTransition(vm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.ToIsEmpty() == true /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return vm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	from := st.msg.From()
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice)
	//fixme:contract debug
	if st.state.GetBalance(from).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(from, mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	from := st.msg.From()

	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(from)
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, contactAddr []byte, err error) {
	msg := st.msg
	sender := vm.AccountRef(msg.From())

<<<<<<< HEAD
=======
	//Initialize account balance, gas + amount
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(msg.Gas()), st.gasPrice)
	st.state.AddBalance(msg.From(), new(big.Int).Add(mgval, msg.Value()))

>>>>>>> 4c17f25eda6f8eefa5bdc69a367db53ccfd879fc
	if err = st.preCheck(); err != nil {
		return nil, 0, nil, err
	}

	contractCreation := msg.ToIsEmpty()

	// Pay intrinsic gas
	rlog.Errorf("ToIsEmpty, %t, to %x, data %x", contractCreation, msg.To(), st.data)
	gas, err := IntrinsicGas(st.data, contractCreation)
	if err != nil {
		return nil, 0, nil, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, nil, err
	}

	var (
		vm = st.vm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)

	var addr common.Address
	if contractCreation {
		rlog.Info("---contractCreation---")
		ret, addr, st.gas, vmerr = vm.Create(sender, st.data, st.gas, st.value)
		rlog.Info("---vmerr---", vmerr)
	} else {
		// Increment the nonce for the next transaction
		rlog.Info("---contractCall---")
		st.state.SetNonce(sender.Address(), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = vm.Call(sender, st.to(), st.data, st.gas, st.value)
		addr = st.to()
		rlog.Info("---vmerr---", vmerr)
	}

	if vmerr != nil {
		rlog.Errorf("VM returned with error: %s", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		// fixme: check if err is vm.ErrInsufficientBalance ???
		return nil, 0, nil, vmerr
	}

	st.refundGas()
	//st.state.AddBalance(st.vm.GetContext().Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	return ret, st.gasUsed(), addr.Bytes(), err
}

func (st *StateTransition) refundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
