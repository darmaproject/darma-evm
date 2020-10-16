// Copyright 2018-2020 Darma Project. All rights reserved.
package walletapi

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"

	"github.com/romana/rlog"
	"github.com/vmihailenco/msgpack"

	"github.com/darmaproject/darmasuite/address"
	"github.com/darmaproject/darmasuite/config"
	"github.com/darmaproject/darmasuite/crypto"
	"github.com/darmaproject/darmasuite/globals"
	"github.com/darmaproject/darmasuite/inputmaturity"
	"github.com/darmaproject/darmasuite/ringct"
	"github.com/darmaproject/darmasuite/structures"
	"github.com/darmaproject/darmasuite/transaction"
)

func (w *Wallet) selectRingMembers(currentInput *ringct.InputInfo, mixin uint64) {
	for {
		posRingCount := 0
		var buf [8]byte
		cryptorand.Read(buf[:])
		r, err := w.load_ring_member(binary.LittleEndian.Uint64(buf[:]) % w.account.IndexGlobal)
		if err == nil {
			// if ring member is not mature, choose another one
			if !inputmaturity.IsInputMature(w.Get_Height(),
				r.Height,
				r.UnlockHeight,
				r.Sigtype) {
				continue
			}

			// make sure ring member are not repeated
			newRingMember := true
			for j := range currentInput.RingMembers { // TODO we donot need the loop
				if r.IndexGlobal == currentInput.RingMembers[j] {
					newRingMember = false // we should not use this ring member
					break
				}
			}
			if !newRingMember {
				continue
			}

			if w.isRingPoS(&r) {
				posRingCount++
				if posRingCount > 2 {
					continue
				}
			}

			// zero amount
			if r.InKey.Mask == ringct.ZeroCommitment {
				rlog.Debugf("index %d is pow reward of 0 amount", r.IndexGlobal)
				continue
			}

			currentInput.RingMembers = append(currentInput.RingMembers, r.IndexGlobal)
			currentInput.Pubs = append(currentInput.Pubs, r.InKey)
		}

		if uint64(len(currentInput.RingMembers)) == mixin { // atleast 5 ring members
			break
		}
	}
}

func (w *Wallet) isRingPoS(r *RingMember) bool {
	switch r.TxType {
	case globals.TX_TYPE_BUY_SHARE:
		fallthrough
	case globals.TX_TYPE_REPO_SHARE:
		fallthrough
	case globals.TX_TYPE_REGISTER_POOL:
		fallthrough
	case globals.TX_TYPE_CLOSE_POOL:
		fallthrough
	case globals.TX_TYPE_POOL_PROFIT:
		fallthrough
	case globals.TX_TYPE_SHARE_PROFIT:
		fallthrough
	case globals.TX_TYPE_BONUS_PROFIT:
		return true
	}

	return false
}

func (w *Wallet) TotalOutput(limit uint64, limitType string, amount uint64) (selectedOutputIndex []uint64, sum uint64) {
	return w.selectOutputsForTransfer(amount, 0, true, limit, limitType, false)
}

func (w *Wallet) Transfer(addr []address.Address, amount []uint64, unlock_time uint64, payment_id_hex string, fees_per_kb uint64, mixin uint64, limit uint64, limitType string) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {
	return w.TransferInternal(addr, amount, unlock_time, payment_id_hex, fees_per_kb, mixin, nil, limit, limitType)
}

func (w *Wallet) TransferV2(addr []address.Address, amount []uint64, unlock_time uint64, payment_id_hex string, fees_per_kb uint64, mixin uint64, tx_extra *transaction.TxCreateExtra) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {
	return w.TransferInternal(addr, amount, unlock_time, payment_id_hex, fees_per_kb, mixin, tx_extra, 0, "")
}

func (w *Wallet) TransferV3(addr []address.Address, amount []uint64, unlock_time uint64, payment_id_hex string, fees_per_kb uint64, mixin uint64, tx_extra *transaction.TxCreateExtra) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {
	return w.TransferInternal(addr, amount, unlock_time, payment_id_hex, fees_per_kb, mixin, tx_extra, 0, "")
}

// send amount to specific addresses
func (w *Wallet) TransferInternal(
	addr []address.Address,
	amount []uint64,
	unlock_time uint64,
	payment_id_hex string,
	fees_per_kb uint64,
	mixin uint64,
	tx_extra *transaction.TxCreateExtra,
	limit uint64,
	limitType string) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {
	var transfer_details structures.Outgoing_Transfer_Details
	isContract := false

	w.transferMutex.Lock()
	defer w.transferMutex.Unlock()
	if mixin == 0 {
		mixin = uint64(w.account.Mixin) // use wallet mixin, if mixin not provided
	}
	if mixin < 5 { // enforce minimum mixin
		mixin = 5
	}

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	//if w.GetMode()  {
	fees_per_kb = w.dynamicFeesPerKb // TODO disabled as protection while lots more testing is going on
	rlog.Infof("Fees per KB %d\n", fees_per_kb)
	//}

	if fees_per_kb == 0 {
		if w.account.Height < uint64(globals.GetVotingStartHeight()) {
			fees_per_kb = config.BEFORE_DPOS_FEE_PER_KB
		} else {
			fees_per_kb = config.FEE_PER_KB
		}
	}

	var txw *TXWalletData
	if len(addr) != len(amount) {
		err = fmt.Errorf("Count of address and amounts mismatch")
		return
	}

	if tx_extra != nil && tx_extra.ContractData != nil {
		isContract = true
	}
	if isContract == false && len(addr) < 1 {
		err = fmt.Errorf("Destination address missing")
		return
	}

	var paymentId []byte // we later on find WHETHER to include it, encrypt it depending on length

	// if payment  ID is provided explicity, use it
	if payment_id_hex != "" {
		paymentId, err = hex.DecodeString(payment_id_hex) // payment_id in hex
		if err != nil {
			return
		}

		if len(paymentId) == 32 || len(paymentId) == 8 {
		} else {
			err = fmt.Errorf("Payment ID must be atleast 64 hex chars (32 bytes) or 16 hex chars 8 byte")
			return
		}
	}

	// only only single payment id
	for i := range addr {
		if addr[i].IsIntegratedAddress() && payment_id_hex != "" {
			err = fmt.Errorf("Payment ID provided in both integrated address and separately")
			return
		}
	}

	// if integrated address payment id present , normal payment id must not be provided
	for i := range addr {
		if !addr[i].IsDARMANetwork() {
			err = fmt.Errorf("address provided is not a valid DMCH network address")
			return
		}
		if addr[i].IsMainnet() != globals.IsMainnet() {
			err = fmt.Errorf("address provided has invalid DMCH network mainnet/testnet")
			return
		}

		if addr[i].IsIntegratedAddress() {
			if len(paymentId) > 0 { // a transaction can have only single encrypted payment ID
				err = fmt.Errorf("More than 1 integrated address provided")
				return
			}
			paymentId = addr[i].PaymentID
		}
	}

	fees := uint64(0) // start with zero fees
	expectedFee := uint64(0)
	totalAmountRequired := uint64(0)
	diff := uint64(0)

	for i := range amount {
		if amount[i] == 0 { // cannot send 0  amount
			err = fmt.Errorf("Sending 0 amount to destination NOT possible")
			return
		}
		totalAmountRequired += amount[i]
	}

	// infinite tries to build a transaction
	for {
		// we need to make sure that account has sufficient unlocked balance ( to send amount ) + required amount of fees
		unlocked, _ := w.GetBalance()

		if totalAmountRequired > unlocked {
			err = fmt.Errorf("Insufficient unlocked balance: %s", globals.FormatMoney(unlocked))
			return
		}

		// now we need to select outputs with sufficient balance
		//total_amount_required += fees
		// select few outputs randomly

		inputs_selected, inputs_sum = w.selectOutputsForTransfer(totalAmountRequired, fees+expectedFee, false, limit, limitType, w.Lightweight_mode)
		if inputs_sum == 0 {
			err = fmt.Errorf("Reading available funds failed, please check your wallet or network")
			return
		}

		if inputs_sum < (totalAmountRequired+fees) && (!w.Lightweight_mode || isContract) {
			err = fmt.Errorf("Insufficient unlocked balance(fee %s)", globals.FormatMoney(fees))
			return
		}

		rlog.Infof("Selected %d (%+v) iinputs to transfer  %s DMCH\n", len(inputs_selected), inputs_selected, globals.FormatMoney(inputs_sum))

		// lets prepare the inputs for ringct, so as we can used them
		var inputs []ringct.InputInfo
		for i := range inputs_selected {
			txw, err = w.loadFundsData(inputs_selected[i], FUNDS_BUCKET)
			if err != nil {
				err = fmt.Errorf("Error while reading available funds index( it was just selected ) index %d err %s", inputs_selected[i], err)
				return
			}

			rlog.Infof("current input  %d %d \n", i, inputs_selected[i])
			var currentInput ringct.InputInfo
			currentInput.Amount = txw.WAmount
			currentInput.Key_image = crypto.Hash(txw.WKimage)
			currentInput.Sk = txw.WKey

			currentInput.Index_Global = txw.TXdata.Index_Global

			// add ring members here
			// TODO force random ring members

			//  mandatory add ourselves as ring member, otherwise there is no point in building the tx
			currentInput.RingMembers = append(currentInput.RingMembers, currentInput.Index_Global)
			currentInput.Pubs = append(currentInput.Pubs, txw.TXdata.InKey)

			// add necessary amount  of random ring members
			// TODO we need to make sure ring members are mature, otherwise tx will fail because o immature inputs
			// This can cause certain TX to fail
			w.selectRingMembers(&currentInput, mixin)

			rlog.Infof(" current input before sorting %+v \n", currentInput.RingMembers)
			currentInput = sortRingMembers(currentInput)
			rlog.Infof(" current input after sorting  %+v \n", currentInput.RingMembers)
			inputs = append(inputs, currentInput)
		}

		// fill in the outputs
		var outputs []ringct.Output_info

	rebuild_tx_with_correct_fee:
		if inputs_sum < (totalAmountRequired + fees) {
			diff = totalAmountRequired + fees - inputs_sum
		}
		outputs = outputs[:0]

		transfer_details.Fees = fees
		transfer_details.Amount = transfer_details.Amount[:0]
		transfer_details.Daddress = transfer_details.Daddress[:0]
		sendamount := uint64(0)
		for i := range addr {
			var output ringct.Output_info
			output.Amount = amount[i] - uint64(diff/uint64(len(addr)))
			output.Public_Spend_Key = addr[i].SpendKey
			output.Public_View_Key = addr[i].ViewKey
			output.ExtraPublicKey = addr[i].IsSubAddress()

			sendamount += output.Amount
			transfer_details.Amount = append(transfer_details.Amount, output.Amount)
			transfer_details.Daddress = append(transfer_details.Daddress, addr[i].String())

			outputs = append(outputs, output)
		}
		transfer_details.SendAmount = sendamount

		// get ready to receive change
		var change ringct.Output_info
		change.Amount = inputs_sum - totalAmountRequired - fees + diff // we must have atleast change >= fees
		change.Public_Spend_Key = w.account.Keys.Spendkey_Public       /// fill our public spend key
		change.Public_View_Key = w.account.Keys.Viewkey_Public         // fill our public view key

		if change.Amount > 0 { // include change only if required
			transfer_details.Amount = append(transfer_details.Amount, change.Amount)
			transfer_details.Daddress = append(transfer_details.Daddress, w.account.GetAddress().String())

			outputs = append(outputs, change)
		}

		changeAmount = change.Amount

		// if encrypted payment ids are used, they are encrypted against first output
		// if we shuffle outputs encrypted ids will break
		if unlock_time == 0 { // shuffle output and change randomly
			if len(paymentId) == 8 { // do not shuffle if encrypted payment IDs are used

			} else {
				if isContract == false {
					globals.Global_Random.Shuffle(len(outputs), func(i, j int) {
						outputs[i], outputs[j] = outputs[j], outputs[i]
					})
				}
			}
		}

		// outputs = append(outputs, change)
		tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, paymentId, true, tx_extra)
		//tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, paymentId, false)

		tx_size := uint64(len(tx.Serialize()))
		size_in_kb := tx_size / 1024

		if (tx_size % 1024) != 0 { // for any part there of, use a full KB fee
			size_in_kb += 1
		}

		minimum_fee := size_in_kb * fees_per_kb

		needed_fee := w.getfees(minimum_fee) // multiply minimum fees by multiplier

		rlog.Infof("minimum fee %s required fees %s provided fee %s size %d fee/kb %s\n", globals.FormatMoney(minimum_fee), globals.FormatMoney(needed_fee), globals.FormatMoney(fees), size_in_kb, globals.FormatMoney(fees_per_kb))

		if fees > needed_fee { // transaction was built up successfully
			fees = needed_fee // setup fees parameter exactly as much required
			goto rebuild_tx_with_correct_fee
		}

		// keep trying until we are successfull or funds become Insufficient
		if fees == needed_fee { // transaction was built up successfully
			break
		}

		// we need to try again
		fees = needed_fee             // setup estimated parameter
		expectedFee = expectedFee * 2 // double the estimated fee
	}

	// log enough information to wallet to display it again to users
	transfer_details.PaymentID = hex.EncodeToString(paymentId)

	// get the tx secret key and store it
	txhash := tx.GetHash()
	transfer_details.TXsecretkey = w.GetTXKey(tx.GetHash())
	transfer_details.TXID = txhash.String()

	// lets marshal the structure and store it in in DB

	details_serialized, err := json.Marshal(transfer_details)
	if err != nil {
		rlog.Warnf("Err marshalling details err %s", err)
	}

	w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(TX_OUT_DETAILS_BUCKET), txhash[:], details_serialized[:])
	{
		rlog.Infof("Transfering total amount %s \n", globals.FormatMoneyPrecision(inputs_sum, 12))
		rlog.Infof("total amount (output) %s \n", globals.FormatMoneyPrecision(totalAmountRequired, 12))
		rlog.Infof("change amount ( will come back ) %s \n", globals.FormatMoneyPrecision(changeAmount, 12))
		rlog.Infof("fees %s \n", globals.FormatMoneyPrecision(tx.RctSignature.GetTXFee(), 12))
		rlog.Infof("Inputs %d == outputs %d ( %d + %d + %d )", inputs_sum, (totalAmountRequired - diff + changeAmount + tx.RctSignature.GetTXFee()), totalAmountRequired-diff, changeAmount, tx.RctSignature.GetTXFee())
		if inputs_sum != (totalAmountRequired + changeAmount + tx.RctSignature.GetTXFee() - diff) {
			rlog.Warnf("INPUTS != OUTPUTS, please check")
			panic(fmt.Sprintf("Inputs %d != outputs ( %d + %d + %d )", inputs_sum, totalAmountRequired, changeAmount, tx.RctSignature.GetTXFee()))
		}
	}

	return
}

// send all unlocked balance amount to specific address
func (w *Wallet) TransferEverything(addr address.Address, payment_id_hex string, unlock_time uint64, fees_per_kb uint64, mixin uint64) (tx *transaction.Transaction, inputs_selected []uint64, inputsSum uint64, err error) {
	var transferDetails structures.Outgoing_Transfer_Details

	w.transferMutex.Lock()
	defer w.transferMutex.Unlock()

	if mixin < 5 { // enforce minimum mixin
		mixin = 5
	}

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	fees_per_kb = w.dynamicFeesPerKb // TODO disabled as protection while lots more testing is going on
	rlog.Infof("Fees per KB %d\n", fees_per_kb)

	if fees_per_kb == 0 { // hard coded at compile time
		if w.account.Height < uint64(globals.GetVotingStartHeight()) {
			fees_per_kb = config.BEFORE_DPOS_FEE_PER_KB
		} else {
			fees_per_kb = config.FEE_PER_KB
		}
	}

	var txw *TXWalletData

	var payment_id []byte // we later on find WHETHER to include it, encrypt it depending on length

	// if payment  ID is provided explicity, use it
	if payment_id_hex != "" {
		payment_id, err = hex.DecodeString(payment_id_hex) // payment_id in hex
		if err != nil {
			return
		}

		if len(payment_id) == 32 || len(payment_id) == 8 {

		} else {
			err = fmt.Errorf("Payment ID must be atleast 64 hex chars (32 bytes) or 16 hex chars 8 byte")
			return
		}

	}

	// only only single payment id
	if addr.IsIntegratedAddress() && payment_id_hex != "" {
		err = fmt.Errorf("Payment ID provided in both integrated address and separately")
		return
	}
	// if integrated address payment id present , normal payment id must not be provided
	if addr.IsIntegratedAddress() {
		payment_id = addr.PaymentID
	}

	fees := uint64(0) // start with zero fees
	expectedFee := uint64(0)

	// infinite tries to build a transaction
	for {
		// now we need to select all outputs with sufficient balance
		inputs_selected, inputsSum = w.selectOutputsForTransfer(0, fees+expectedFee, true, 0, "", false)

		if len(inputs_selected) < 1 {
			err = fmt.Errorf("Insufficient unlocked balance")
			return
		}

		rlog.Infof("Selected %d (%+v) iinputs to transfer  %s DMCH\n", len(inputs_selected), inputs_selected, globals.FormatMoney(inputsSum))

		// lets prepare the inputs for ringct, so as we can used them
		var inputs []ringct.InputInfo
		for i := range inputs_selected {
			txw, err = w.loadFundsData(inputs_selected[i], FUNDS_BUCKET)
			if err != nil {
				err = fmt.Errorf("Error while reading available funds index( it was just selected ) index %d err %s", inputs_selected[i], err)
				return
			}

			rlog.Infof("current input  %d %d \n", i, inputs_selected[i])
			var currentInput ringct.InputInfo
			currentInput.Amount = txw.WAmount
			currentInput.Key_image = crypto.Hash(txw.WKimage)
			currentInput.Sk = txw.WKey

			//current_input.Index = i   is calculated after sorting of ring members
			currentInput.Index_Global = txw.TXdata.Index_Global

			//  mandatory add ourselves as ring member, otherwise there is no point in building the tx
			currentInput.RingMembers = append(currentInput.RingMembers, currentInput.Index_Global)
			currentInput.Pubs = append(currentInput.Pubs, txw.TXdata.InKey)

			// add necessary amount  of random ring members
			// TODO we need to make sure ring members are mature, otherwise tx will fail because o immature inputs
			// This can cause certain TX to fail
			w.selectRingMembers(&currentInput, mixin)

			rlog.Infof(" current input before sorting %+v \n", currentInput.RingMembers)
			currentInput = sortRingMembers(currentInput)
			rlog.Infof(" current input after sorting  %+v \n", currentInput.RingMembers)
			inputs = append(inputs, currentInput)
		}

		// fill in the outputs
		var outputs []ringct.Output_info
	REBUILD_TX_WITH_CORRECT_FEE:
		outputs = outputs[:0]

		var output ringct.Output_info
		output.Amount = inputsSum - fees
		output.Public_Spend_Key = addr.SpendKey
		output.Public_View_Key = addr.ViewKey
		output.ExtraPublicKey = addr.IsSubAddress()

		transferDetails.Fees = fees
		transferDetails.Amount = transferDetails.Amount[:0]
		transferDetails.Daddress = transferDetails.Daddress[:0]

		transferDetails.Amount = append(transferDetails.Amount, output.Amount)
		transferDetails.Daddress = append(transferDetails.Daddress, addr.String())
		transferDetails.SendAmount = output.Amount
		outputs = append(outputs, output)

		// outputs = append(outputs, change)
		tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, payment_id, true, nil)
		//tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, payment_id, false)

		txSize := uint64(len(tx.Serialize()))
		sizeInKb := txSize / 1024

		if (txSize % 1024) != 0 { // for any part there of, use a full KB fee
			sizeInKb++
		}

		minimumFee := sizeInKb * fees_per_kb
		neededFee := w.getfees(minimumFee) // multiply minimum fees by multiplier

		rlog.Infof("required fees %s provided fee %s size %d fee/kb %s\n", globals.FormatMoney(neededFee), globals.FormatMoney(fees), sizeInKb, globals.FormatMoney(fees_per_kb))
		if inputsSum <= fees {
			err = fmt.Errorf("Insufficient unlocked balance to cover fees")
			return
		}

		if fees > neededFee { // transaction was built up successfully
			fees = neededFee // setup fees parameter exactly as much required
			goto REBUILD_TX_WITH_CORRECT_FEE
		}

		// keep trying until we are successfull or funds become Insufficient
		if fees == neededFee { // transaction was built up successfully
			break
		}

		// we need to try again
		fees = neededFee              // setup estimated parameter
		expectedFee = expectedFee * 2 // double the estimated fee
	}

	// log enough information to wallet to display it again to users
	transferDetails.PaymentID = hex.EncodeToString(payment_id)

	// get the tx secret key and store it
	txhash := tx.GetHash()
	transferDetails.TXsecretkey = w.GetTXKey(tx.GetHash())
	transferDetails.TXID = txhash.String()

	// lets marshal the structure and store it in in DB

	detailsSerialized, err := json.Marshal(transferDetails)
	if err != nil {
		rlog.Warnf("Err marshalling details err %s", err)
	}

	w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(TX_OUT_DETAILS_BUCKET), txhash[:], detailsSerialized[:])

	return
}

func (w *Wallet) TransferLocked(addr address.Address, amount uint64, payment_id_hex string, unlock_time uint64, fees_per_kb uint64, mixin uint64, tx_extra *transaction.TxCreateExtra, limit uint64, limitType string) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {

	var transfer_details structures.Outgoing_Transfer_Details
	w.transferMutex.Lock()
	defer w.transferMutex.Unlock()
	if mixin == 0 {
		mixin = uint64(w.account.Mixin) // use wallet mixin, if mixin not provided
	}
	if mixin < 5 { // enforce minimum mixin
		mixin = 5
	}

	// if wallet is online,take the fees from the network itself
	// otherwise use whatever user has provided
	//if w.GetMode()  {
	fees_per_kb = w.dynamicFeesPerKb // TODO disabled as protection while lots more testing is going on
	rlog.Infof("Fees per KB %d\n", fees_per_kb)
	//}

	if fees_per_kb == 0 {
		if w.account.Height < uint64(globals.GetVotingStartHeight()) {
			fees_per_kb = config.BEFORE_DPOS_FEE_PER_KB
		} else {
			fees_per_kb = config.FEE_PER_KB
		}
	}

	if tx_extra == nil {
		err = fmt.Errorf("Lock type must be specified.")
		return
	}

	var txw *TXWalletData
	var paymentId []byte // we later on find WHETHER to include it, encrypt it depending on length

	// if payment  ID is provided explicity, use it
	if payment_id_hex != "" {
		paymentId, err = hex.DecodeString(payment_id_hex) // payment_id in hex
		if err != nil {
			return
		}

		if len(paymentId) == 32 || len(paymentId) == 8 {
		} else {
			err = fmt.Errorf("Payment ID must be atleast 64 hex chars (32 bytes) or 16 hex chars 8 byte")
			return
		}
	}

	// only only single payment id
	if addr.IsIntegratedAddress() && payment_id_hex != "" {
		err = fmt.Errorf("Payment ID provided in both integrated address and separately")
		return
	}

	// if integrated address payment id present , normal payment id must not be provided
	if addr.IsIntegratedAddress() {
		if len(paymentId) > 0 { // a transaction can have only single encrypted payment ID
			err = fmt.Errorf("More than 1 integrated address provided")
			return
		}
		paymentId = addr.PaymentID
	}

	fees := uint64(0) // start with zero fees
	expectedFee := uint64(0)
	totalAmountRequired := uint64(0)

	if tx_extra.LockedType == transaction.LOCKEDTYPE_LOCKED && amount == 0 { // cannot send 0  amount
		err = fmt.Errorf("Sending 0 amount to destination NOT possible")
		return
	}
	totalAmountRequired = amount

	var lockedAmount uint64
	switch tx_extra.VoutTarget.(type) {
	case transaction.TxoutToRegisterPool:
		lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToRegisterPool).Value
	case transaction.TxoutToBuyShare:
		lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToBuyShare).Value
	default:
	}

	// infinite tries to build a transaction
	for {
		if tx_extra.LockedType == transaction.LOCKEDTYPE_LOCKED {
			// we need to make sure that account has sufficient unlocked balance ( to send amount ) + required amount of fees
			unlocked, _ := w.GetBalance()
			if totalAmountRequired >= unlocked {
				err = fmt.Errorf("Insufficient unlocked balance %s", globals.FormatMoney(unlocked))
				return
			}

			// now we need to select outputs with sufficient balance
			//total_amount_required += fees
			// select few outputs randomly
			inputs_selected, inputs_sum = w.selectOutputsForTransfer(totalAmountRequired, fees+expectedFee, false, limit, limitType, false)
		} else if tx_extra.LockedType == transaction.LOCKEDTYPE_UNLOCKED {
			inputs_selected, inputs_sum, err = w.selectOutputsForUnlocked(tx_extra.LockedTxId)
			if err != nil {
				rlog.Errorf("failed select outputs for unlock, err %s", err)
				return
			}
			amount = inputs_sum - fees
			totalAmountRequired = amount
		}

		if inputs_sum == 0 {
			err = fmt.Errorf("Reading available funds failed, please check your wallet or network")
			return
		}

		if inputs_sum < (totalAmountRequired + fees) {
			err = fmt.Errorf("Insufficient unlocked balance %s", globals.FormatMoney(inputs_sum))
			return
		}

		rlog.Infof("Selected %d (%+v) inputs to transfer %s DMCH\n", len(inputs_selected), inputs_selected, globals.FormatMoney(inputs_sum))

		// lets prepare the inputs for ringct, so as we can used them
		var inputs []ringct.InputInfo
		for i := range inputs_selected {
			txw, err = w.loadFundsData(inputs_selected[i], FUNDS_BUCKET)
			if err != nil {
				err = fmt.Errorf("Error while reading available funds index( it was just selected ) index %d err %s", inputs_selected[i], err)
				return
			}

			rlog.Infof("current input  %d %d \n", i, inputs_selected[i])
			var currentInput ringct.InputInfo
			currentInput.Amount = txw.WAmount
			currentInput.Key_image = crypto.Hash(txw.WKimage)
			currentInput.Sk = txw.WKey

			currentInput.Index_Global = txw.TXdata.Index_Global

			// add ring members here
			// TODO force random ring members

			//  mandatory add ourselves as ring member, otherwise there is no point in building the tx
			currentInput.RingMembers = append(currentInput.RingMembers, currentInput.Index_Global)
			currentInput.Pubs = append(currentInput.Pubs, txw.TXdata.InKey)

			// add necessary amount  of random ring members
			// TODO we need to make sure ring members are mature, otherwise tx will fail because o immature inputs
			// This can cause certain TX to fail
			w.selectRingMembers(&currentInput, mixin)

			rlog.Infof(" current input before sorting %+v \n", currentInput.RingMembers)
			currentInput = sortRingMembers(currentInput)
			rlog.Infof(" current input after sorting  %+v \n", currentInput.RingMembers)
			inputs = append(inputs, currentInput)
		}

		// fill in the outputs
		var outputs []ringct.Output_info

	rebuild_tx_with_correct_fee:
		outputs = outputs[:0]

		transfer_details.Fees = fees
		transfer_details.Amount = transfer_details.Amount[:0]
		transfer_details.Daddress = transfer_details.Daddress[:0]

		var output ringct.Output_info
		if tx_extra.LockedType == transaction.LOCKEDTYPE_UNLOCKED {
			output.Amount = inputs_sum - fees
		} else {
			output.Amount = amount
		}

		if lockedAmount != 0 {
			output.Amount = 0
		}

		output.Public_Spend_Key = addr.SpendKey
		output.Public_View_Key = addr.ViewKey
		output.Vout_Target = tx_extra.VoutTarget
		output.ExtraPublicKey = addr.IsSubAddress()

		if lockedAmount != 0 {
			transfer_details.Amount = append(transfer_details.Amount, lockedAmount)
		} else {
			transfer_details.Amount = append(transfer_details.Amount, output.Amount)
		}

		transfer_details.Daddress = append(transfer_details.Daddress, addr.String())
		transfer_details.SendAmount = transfer_details.Amount[0]
		outputs = append(outputs, output)

		// get ready to receive change
		var change ringct.Output_info
		if lockedAmount != 0 {
			change.Amount = inputs_sum - lockedAmount - fees // we must have atleast change >= fees
		} else {
			change.Amount = inputs_sum - output.Amount - fees // we must have atleast change >= fees
		}

		change.Public_Spend_Key = w.account.Keys.Spendkey_Public /// fill our public spend key
		change.Public_View_Key = w.account.Keys.Viewkey_Public   // fill our public view key

		if change.Amount > 0 { // include change only if required
			transfer_details.Amount = append(transfer_details.Amount, change.Amount)
			transfer_details.Daddress = append(transfer_details.Daddress, w.account.GetAddress().String())

			outputs = append(outputs, change)
		}

		changeAmount = change.Amount

		// outputs = append(outputs, change)
		tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, paymentId, true, tx_extra)
		//tx = w.CreateTXv2(inputs, outputs, fees, unlock_time, paymentId, false)

		tx_size := uint64(len(tx.Serialize()))
		size_in_kb := tx_size / 1024

		if (tx_size % 1024) != 0 { // for any part there of, use a full KB fee
			size_in_kb += 1
		}

		minimum_fee := size_in_kb * fees_per_kb

		needed_fee := w.getfees(minimum_fee) // multiply minimum fees by multiplier

		rlog.Infof("minimum fee %s required fees %s provided fee %s size %d fee/kb %s\n", globals.FormatMoney(minimum_fee), globals.FormatMoney(needed_fee), globals.FormatMoney(fees), size_in_kb, globals.FormatMoney(fees_per_kb))

		if fees > needed_fee { // transaction was built up successfully
			fees = needed_fee // setup fees parameter exactly as much required
			goto rebuild_tx_with_correct_fee
		}

		// keep trying until we are successfull or funds become Insufficient
		if fees == needed_fee { // transaction was built up successfully
			break
		}

		// we need to try again
		fees = needed_fee             // setup estimated parameter
		expectedFee = expectedFee * 2 // double the estimated fee
	}

	// log enough information to wallet to display it again to users
	transfer_details.PaymentID = hex.EncodeToString(paymentId)

	// get the tx secret key and store it
	txhash := tx.GetHash()
	transfer_details.TXsecretkey = w.GetTXKey(tx.GetHash())
	transfer_details.TXID = txhash.String()

	// lets marshal the structure and store it in in DB

	details_serialized, err := json.Marshal(transfer_details)
	if err != nil {
		rlog.Warnf("Err marshalling details err %s", err)
	}

	w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(TX_OUT_DETAILS_BUCKET), txhash[:], details_serialized[:])
	{
		rlog.Infof("Transfering total amount %s \n", globals.FormatMoneyPrecision(inputs_sum, 12))
		rlog.Infof("total amount (output) %s \n", globals.FormatMoneyPrecision(totalAmountRequired, 12))
		rlog.Infof("change amount ( will come back ) %s \n", globals.FormatMoneyPrecision(changeAmount, 12))
		rlog.Infof("fees %s \n", globals.FormatMoneyPrecision(tx.RctSignature.GetTXFee(), 12))
		rlog.Infof("Inputs %d == outputs %d ( %d + %d + %d )", inputs_sum, (totalAmountRequired + changeAmount + tx.RctSignature.GetTXFee()), totalAmountRequired, changeAmount, tx.RctSignature.GetTXFee())
		if inputs_sum != (totalAmountRequired + changeAmount + tx.RctSignature.GetTXFee()) {
			rlog.Warnf("INPUTS != OUTPUTS, please check")
			panic(fmt.Sprintf("Inputs %d != outputs ( %d + %d + %d )", inputs_sum, totalAmountRequired, changeAmount, tx.RctSignature.GetTXFee()))
		}
	}

	return
}

type member struct {
	index uint64
	key   ringct.CtKey
}

type members []member

func (s members) Len() int {
	return len(s)
}

func (s members) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s members) Less(i, j int) bool {
	return s[i].index < s[j].index
}

// sort ring members
func sortRingMembers(input ringct.InputInfo) ringct.InputInfo {
	if len(input.RingMembers) != len(input.Pubs) {
		panic(fmt.Sprintf("Internal error !!!, ring member count %d != pubs count %d", len(input.RingMembers), len(input.Pubs)))
	}

	var dataSet members
	for i := range input.Pubs {
		dataSet = append(dataSet, member{input.RingMembers[i], input.Pubs[i]})
	}
	sort.Sort(dataSet)

	for i := range input.Pubs {
		input.RingMembers[i] = dataSet[i].index
		input.Pubs[i] = dataSet[i].key
		if dataSet[i].index == input.Index_Global {
			input.Index = i
		}
	}

	return input
}

func (w *Wallet) selectOutputsForTransfer(neededAmount uint64, fees uint64, all bool, limit uint64, limitType string, maxinput bool) (selectedOutputIndex []uint64, sum uint64) {
	indexList := w.loadAllValuesFromBucket(BLOCKCHAIN_UNIVERSE, []byte(FUNDS_AVAILABLE))

	// shuffle the index_list
	for i := len(indexList) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indexList[i], indexList[j] = indexList[j], indexList[i]
	}

	pendingKeyImage := w.getAllPendingKeyImage()

	txs := make([]*TXWalletData, 0)
	keyImages := make([]crypto.Key, 0)

	for i := range indexList { // load index
		currentIndex := binary.BigEndian.Uint64(indexList[i])

		tx, err := w.loadFundsData(currentIndex, FUNDS_BUCKET)
		if err != nil {
			rlog.Warnf("Error while reading available funds index index %d err %s", currentIndex, err)
			continue
		}

		if limit > 0 && (limitType == "max" && tx.WAmount > limit) || (limitType == "min" && tx.WAmount < limit) {
			continue
		}

		txs = append(txs, tx)
		keyImages = append(keyImages, tx.WKimage)
	}

	keyImageSpent := w.IsKeyImageSpentBatch(keyImages)
	count := 1
	for _, tx := range txs { // load index
		//TODO remove
		if limit > 0 && (limitType == "max" && tx.WAmount > limit) || (limitType == "min" && tx.WAmount < limit) {
			continue
		}

		if inputmaturity.IsInputMature(w.Get_Height(),
			tx.TXdata.Height,
			tx.TXdata.Unlock_Height,
			tx.TXdata.SigType) && !keyImageSpent[tx.WKimage] && !pendingKeyImage[tx.WKimage] {

			sum += tx.WAmount

			selectedOutputIndex = append(selectedOutputIndex, tx.TXdata.Index_Global) // select this output
			if !all {                                                                 // user requested all inputs
				if count >= config.MAX_INPUT_SIZE && maxinput {
					return
				}
				count++
				if sum > (neededAmount + fees) {
					return
				}
			} else {
				if neededAmount != 0 && sum > (neededAmount+fees) {
					return
				}
			}
		}
	}

	return
}

func (w *Wallet) selectTokenOutputsForTransfer(neededAmount uint64, fees uint64, all bool, limit uint64, limitType string, maxinput bool, tokenId crypto.Hash) (selectedOutputIndex []uint64, sum uint64) {
	indexList := w.loadAllValuesFromBucket(BLOCKCHAIN_UNIVERSE, []byte(TOKEN_FUNDS_AVAILABLE))

	// shuffle the index_list
	for i := len(indexList) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		indexList[i], indexList[j] = indexList[j], indexList[i]
	}

	pendingKeyImage := w.getAllPendingTokenKeyImage()

	txs := make([]*TXWalletData, 0)
	keyImages := make([]crypto.Key, 0)

	for i := range indexList { // load index
		currentIndex := binary.BigEndian.Uint64(indexList[i])

		tx, err := w.loadFundsData(currentIndex, TOKEN_FUNDS_BUCKET)
		if err != nil {
			rlog.Warnf("Error while reading available funds index index %d err %s", currentIndex, err)
			continue
		}

		localTokenId := globals.GetTokenId(tx.TokenSymbol)
		if !bytes.Equal(localTokenId[:], tokenId[:]) {
			continue
		}

		txs = append(txs, tx)
		keyImages = append(keyImages, tx.WKimage)
	}

	keyImageSpent := w.IsTokenKeyImageSpentBatch(keyImages)
	count := 1
	for _, tx := range txs { // load index

		if limit > 0 && (limitType == "max" && tx.WAmount > limit) || (limitType == "min" && tx.WAmount < limit) {
			continue
		}

		if inputmaturity.IsInputMature(w.Get_Height(),
			tx.TXdata.Height,
			tx.TXdata.Unlock_Height,
			tx.TXdata.SigType) && !keyImageSpent[tx.WKimage] && !pendingKeyImage[tx.WKimage] {

			sum += tx.WAmount

			selectedOutputIndex = append(selectedOutputIndex, tx.TXdata.Index_Global) // select this output
			if !all {                                                                 // user requested all inputs
				if count >= config.MAX_INPUT_SIZE && maxinput {
					return
				}
				count++
				if sum > (neededAmount + fees) {
					return
				}
			} else {
				if neededAmount != 0 && sum > (neededAmount+fees) {
					return
				}
			}
		}
	}

	return
}

func (w *Wallet) selectOutputsForUnlocked(txid crypto.Hash) (selectedOutputIndex []uint64, sum uint64, err error) {
	indexList := w.loadAllValuesFromBucket(BLOCKCHAIN_UNIVERSE, []byte(FUNDS_AVAILABLE))

	for i := range indexList { // load index
		currentIndex := binary.BigEndian.Uint64(indexList[i])

		tx, err1 := w.loadFundsData(currentIndex, FUNDS_BUCKET)
		if err1 != nil {
			rlog.Warnf("Error while reading available funds index index %d err %s", currentIndex, err1)
			continue
		}

		if txid != tx.TXdata.TXID || tx.TXdata.Unlock_Height != config.MAX_TX_AMOUNT_UNLOCK {
			continue
		}

		// TODO: maturity
		if w.Get_Height() < tx.TXdata.Height+config.NORMAL_TX_AMOUNT_UNLOCK {
			err = fmt.Errorf("Locked tx %s is not mature", txid)
			return
		}

		selectedOutputIndex = append(selectedOutputIndex, currentIndex)
		sum = tx.WAmount
		break
	}

	if len(selectedOutputIndex) != 1 {
		err = fmt.Errorf("Unlock tx %s not found.", txid)
	}
	return
}

func (w *Wallet) selectTokenOutputsForUnlocked(txid crypto.Hash) (selectedOutputIndex []uint64, sum uint64, err error) {
	indexList := w.loadAllValuesFromBucket(BLOCKCHAIN_UNIVERSE, []byte(TOKEN_FUNDS_AVAILABLE))

	for i := range indexList { // load index
		currentIndex := binary.BigEndian.Uint64(indexList[i])

		tx, err1 := w.loadFundsData(currentIndex, FUNDS_BUCKET)
		if err1 != nil {
			rlog.Warnf("Error while reading available funds index index %d err %s", currentIndex, err1)
			continue
		}

		if txid != tx.TXdata.TXID || tx.TXdata.Unlock_Height != config.MAX_TX_AMOUNT_UNLOCK {
			continue
		}

		// TODO: maturity
		if w.Get_Height() < tx.TXdata.Height+config.NORMAL_TX_AMOUNT_UNLOCK {
			err = fmt.Errorf("Locked tx %s is not mature", txid)
			return
		}

		selectedOutputIndex = append(selectedOutputIndex, currentIndex)
		sum = tx.WAmount
		break
	}

	if len(selectedOutputIndex) != 1 {
		err = fmt.Errorf("Unlock tx %s not found.", txid)
	}
	return
}

// load funds data structure from DB
func (w *Wallet) loadFundsData(index uint64, bucket string) (tx_wallet *TXWalletData, err error) {
	valueBytes, err := w.loadKeyValue(BLOCKCHAIN_UNIVERSE, []byte(bucket), itob(index))
	if err != nil {
		err = fmt.Errorf("Error while reading available funds index index %d err %s", index, err)
		return
	}

	tx_wallet = &TXWalletData{}
	err = msgpack.Unmarshal(valueBytes, &tx_wallet)
	if err != nil {
		err = fmt.Errorf("Error while decoding availble funds data index %d err %s", index, err)
		tx_wallet = nil
		return
	}
	return // everything was success
}

func (w *Wallet) loadTokenWalletData(index uint64, bucket string) (token_wallet_data *TokenWalletData, err error) {
	valueBytes, err := w.loadKeyValue(BLOCKCHAIN_UNIVERSE, []byte(bucket), itob(index))
	if err != nil {
		err = fmt.Errorf("Error while reading available token funds index index %d err %s", index, err)
		return
	}

	token_wallet_data = &TokenWalletData{}
	err = msgpack.Unmarshal(valueBytes, &token_wallet_data)
	if err != nil {
		err = fmt.Errorf("Error while decoding availble token funds data index %d err %s", index, err)
		token_wallet_data = nil
		return
	}
	return // everything was success
}

// this will create ringct simple 2 transaction to transfer x amount
func (w *Wallet) CreateTXv2(inputs []ringct.InputInfo, outputs []ringct.Output_info, fees uint64, unlock_time uint64, paymentId []byte, bulletproof bool, tx_extra *transaction.TxCreateExtra) (txout *transaction.Transaction) {
	var tx transaction.Transaction
	tx.Version = config.TX_VERSION_NORMAL
	tx.UnlockTime = unlock_time // for the first input

	// setup the vins as they should be , setup key image
	for i := range inputs {
		txin := transaction.TxinToKey{Amount: 0, K_image: inputs[i].Key_image} //amount is always zero in ringct and later

		if len(inputs[i].RingMembers) != len(inputs[i].Pubs) {
			panic(fmt.Sprintf("Ring members and public keys should be equal %d %d", len(inputs[i].RingMembers), len(inputs[i].Pubs)))
		}
		// fill in the ring members coded as offsets
		lastMember := uint64(0)
		for j := range inputs[i].RingMembers {
			currentOffset := inputs[i].RingMembers[j] - lastMember
			lastMember = inputs[i].RingMembers[j]
			txin.KeyOffsets = append(txin.KeyOffsets, currentOffset)
		}

		tx.Vin = append(tx.Vin, txin)
	}

	// input setup is completed, now we need to setup outputs
	// generate transaction wide unique key
	txSecretKey, txPublicKey := crypto.NewKeyPair() // create new tx key pair

	tx.ExtraMap = map[transaction.EXTRA_TAG]interface{}{}
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = *txPublicKey
	tx.PaymentIDMap = map[transaction.EXTRA_TAG]interface{}{}

	//append locked extra
	if tx_extra != nil {
		if tx_extra.LockedType != transaction.LOCKEDTYPE_NONE {
			tx.ExtraMap[transaction.TX_EXTRA_LOCKED] = &transaction.TransactionLocked{
				LockType: tx_extra.LockedType,
			}
		}
		if tx_extra.ContractData != nil {
			tx.ExtraMap[transaction.TX_EXTRA_CONTRACT] = tx_extra.ContractData
		}
	}

	var lockedAmount uint64
	if tx_extra != nil && tx_extra.VoutTarget != nil {
		switch tx_extra.VoutTarget.(type) {
		case transaction.TxoutToRegisterPool:
			lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToRegisterPool).Value
		case transaction.TxoutToBuyShare:
			lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToBuyShare).Value
		default:
		}
	}

	if lockedAmount != 0 {
		tx.Version = config.TX_VERSION_LOCKED
	}

	if len(paymentId) == 32 {
		tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID] = paymentId
	}

	for i := range outputs {
		derivation := crypto.KeyDerivation(&outputs[i].Public_View_Key, txSecretKey) // keyderivation using output address view key
		// payment id if encrypted are encrypted against first receipient
		if i == 0 { // encrypt it now for the first output
			if len(paymentId) == 8 { // it is an encrypted payment ID,
				tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID] = EncryptDecryptPaymentID(derivation, *txPublicKey, paymentId)
			}
		}

		// this becomes the key within Vout
		indexWithinTx := i
		ehphermalPublicKey := derivation.KeyDerivationToPublicKey(uint64(indexWithinTx), outputs[i].Public_Spend_Key)
		switch outputs[i].Vout_Target.(type) {
		case transaction.TxoutToRegisterPool:
			target := outputs[i].Vout_Target.(transaction.TxoutToRegisterPool)
			target.Key = ehphermalPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		case transaction.TxoutToClosePool:
			target := outputs[i].Vout_Target.(transaction.TxoutToClosePool)
			target.Key = ehphermalPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		case transaction.TxoutToBuyShare:
			target := outputs[i].Vout_Target.(transaction.TxoutToBuyShare)
			target.Key = ehphermalPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		case transaction.TxoutToRepoShare:
			target := outputs[i].Vout_Target.(transaction.TxoutToRepoShare)
			target.Key = ehphermalPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		default:
			if outputs[i].ExtraPublicKey {
				target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(txSecretKey, &outputs[i].Public_Spend_Key)}
				target.Key = ehphermalPublicKey
				tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
			} else {
				tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ehphermalPublicKey}})
			}
		}

		// setup key so as output amount can be encrypted, this will be passed later on to ringct package to encrypt amount
		outputs[i].Scalar_Key = *(derivation.KeyDerivationToScalar(uint64(indexWithinTx)))
	}
	tx.Extra = tx.SerializeExtra() // serialize the extra

	// now comes the ringct part, we always generate rinct simple, they are a bit larger (~1KB) if only single input is used
	// but that is okay as soon we will migrate to bulletproof
	tx.RctSignature = &ringct.RctSig{} // we always generate ringct simple

	if bulletproof {
		if !globals.IsMainnet() || w.Get_Height() >= config.FIX_CLSAG {
			tx.RctSignature.Gen_RingCT_Simple_BulletProof(tx.GetPrefixHash(), inputs, outputs, fees, lockedAmount, ringct.RCTTypeCLSAG)
		} else {
			tx.RctSignature.Gen_RingCT_Simple_BulletProof(tx.GetPrefixHash(), inputs, outputs, fees, lockedAmount, ringct.RCTTypeSimpleBulletproof)
		}
	} else {
		tx.RctSignature.Gen_RingCT_Simple(tx.GetPrefixHash(), inputs, outputs, fees)
	}

	// store the tx key to db, always, since we will never no since the tx may be sent offline
	txhash := tx.GetHash()
	w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(SECRET_KEY_BUCKET), txhash[:], txSecretKey[:])

	return &tx
}

func (w *Wallet) CreateTXv3(inputs []ringct.InputInfo, outputs []ringct.Output_info, fees uint64, unlock_time uint64, paymentId []byte, bulletproof bool, tx_extra *transaction.TxCreateExtra, omniToken *transaction.OmniToken, tokenTx *transaction.Transaction) (txout *transaction.Transaction) {
	var tx transaction.Transaction
	tx.Version = config.TX_VERSION_NORMAL
	tx.UnlockTime = unlock_time // for the first input

	// setup the vins as they should be , setup key image
	for i := range inputs {
		txin := transaction.TxinToKey{Amount: 0, K_image: inputs[i].Key_image} //amount is always zero in ringct and later

		if len(inputs[i].RingMembers) != len(inputs[i].Pubs) {
			panic(fmt.Sprintf("Ring members and public keys should be equal %d %d", len(inputs[i].RingMembers), len(inputs[i].Pubs)))
		}
		// fill in the ring members coded as offsets
		lastMember := uint64(0)
		for j := range inputs[i].RingMembers {
			currentOffset := inputs[i].RingMembers[j] - lastMember
			lastMember = inputs[i].RingMembers[j]
			txin.KeyOffsets = append(txin.KeyOffsets, currentOffset)
		}

		tx.Vin = append(tx.Vin, txin)
	}

	// input setup is completed, now we need to setup outputs
	// generate transaction wide unique key
	//var txPublicKey *crypto.Key
	//if txSecretKey == nil {
	txSecretKey, txPublicKey := crypto.NewKeyPair() // create new tx key pair
	//} else {
	//	txPublicKey = txSecretKey.PublicKey()
	//}

	tx.ExtraMap = map[transaction.EXTRA_TAG]interface{}{}
	tx.ExtraMap[transaction.TX_PUBLIC_KEY] = *txPublicKey
	tx.PaymentIDMap = map[transaction.EXTRA_TAG]interface{}{}

	//append locked extra
	//if tx_extra != nil {
	//	if tx_extra.LockedType != transaction.LOCKEDTYPE_NONE {
	//		tx.ExtraMap[transaction.TX_EXTRA_LOCKED] = &transaction.TransactionLocked{
	//			LockType: tx_extra.LockedType,
	//		}
	//	}
	//	if tx_extra.ContractData != nil {
	//		tx.ExtraMap[transaction.TX_EXTRA_CONTRACT] = tx_extra.ContractData
	//	}
	//}

	var lockedAmount uint64
	//if tx_extra != nil && tx_extra.VoutTarget != nil {
	//	switch tx_extra.VoutTarget.(type) {
	//	case transaction.TxoutToRegisterPool:
	//		lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToRegisterPool).Value
	//	case transaction.TxoutToBuyShare:
	//		lockedAmount = tx_extra.VoutTarget.(transaction.TxoutToBuyShare).Value
	//	default:
	//	}
	//}

	if omniToken != nil && omniToken.Flag == globals.ISSUE_TOKEN {
		lockedAmount = globals.StringCost(omniToken.Symbol)
		tx.Version = config.TX_VERSION_LOCKED
	}

	if omniToken != nil && omniToken.Flag > 0 {
		tokenTxBytes := tokenTx.Serialize()
		tx.ExtraMap[transaction.TOKEN_TX] = tokenTxBytes

		tx.ExtraMap[transaction.OMNI_TOKEN] = omniToken
	}

	if len(paymentId) == 32 {
		tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_PAYMENT_ID] = paymentId
	}

	for i := range outputs {
		derivation := crypto.KeyDerivation(&outputs[i].Public_View_Key, txSecretKey) // keyderivation using output address view key
		// payment id if encrypted are encrypted against first receipient
		if i == 0 { // encrypt it now for the first output
			if len(paymentId) == 8 { // it is an encrypted payment ID,
				tx.PaymentIDMap[transaction.TX_EXTRA_NONCE_ENCRYPTED_PAYMENT_ID] = EncryptDecryptPaymentID(derivation, *txPublicKey, paymentId)
			}
		}

		// this becomes the key within Vout
		indexWithinTx := i
		ehphermalPublicKey := derivation.KeyDerivationToPublicKey(uint64(indexWithinTx), outputs[i].Public_Spend_Key)
		//switch outputs[i].Vout_Target.(type) {
		//case transaction.TxoutToRegisterPool:
		//	target := outputs[i].Vout_Target.(transaction.TxoutToRegisterPool)
		//	target.Key = ehphermalPublicKey
		//	tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		//case transaction.TxoutToClosePool:
		//	target := outputs[i].Vout_Target.(transaction.TxoutToClosePool)
		//	target.Key = ehphermalPublicKey
		//	tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		//case transaction.TxoutToBuyShare:
		//	target := outputs[i].Vout_Target.(transaction.TxoutToBuyShare)
		//	target.Key = ehphermalPublicKey
		//	tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		//case transaction.TxoutToRepoShare:
		//	target := outputs[i].Vout_Target.(transaction.TxoutToRepoShare)
		//	target.Key = ehphermalPublicKey
		//	tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		//default:
		if outputs[i].ExtraPublicKey {
			target := transaction.TxoutToSubAddress{PubKey: *crypto.NewKeyByPoint(txSecretKey, &outputs[i].Public_Spend_Key)}
			target.Key = ehphermalPublicKey
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: target})
		} else {
			tx.Vout = append(tx.Vout, transaction.TxOut{Amount: 0, Target: transaction.TxoutToKey{Key: ehphermalPublicKey}})
		}
		//}

		// setup key so as output amount can be encrypted, this will be passed later on to ringct package to encrypt amount
		outputs[i].Scalar_Key = *(derivation.KeyDerivationToScalar(uint64(indexWithinTx)))
	}
	tx.Extra = tx.SerializeExtra() // serialize the extra

	// now comes the ringct part, we always generate rinct simple, they are a bit larger (~1KB) if only single input is used
	// but that is okay as soon we will migrate to bulletproof
	tx.RctSignature = &ringct.RctSig{} // we always generate ringct simple

	if bulletproof {
		tx.RctSignature.Gen_RingCT_Simple_BulletProof(tx.GetPrefixHash(), inputs, outputs, fees, lockedAmount, ringct.RCTTypeCLSAG)
	} else {
		tx.RctSignature.Gen_RingCT_Simple(tx.GetPrefixHash(), inputs, outputs, fees)
	}

	// store the tx key to db, always, since we will never no since the tx may be sent offline
	txhash := tx.GetHash()
	if omniToken != nil && len(omniToken.Symbol) != 0 && omniToken.Flag > 0 { // TokenTx transfer token.
		w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(TOKEN_SECRET_KEY_BUCKET), txhash[:], txSecretKey[:])
	} else {
		w.storeKeyValue(BLOCKCHAIN_UNIVERSE, []byte(SECRET_KEY_BUCKET), txhash[:], txSecretKey[:])
	}

	return &tx
}

func (w *Wallet) BuildContractTx(code []byte, amount, gas, gasPrice uint64, contractAddr string, isCreate bool) (tx *transaction.Transaction, inputs_selected []uint64, inputs_sum uint64, changeAmount uint64, err error) {
	if gasPrice < config.MIN_GASPRICE {
		rlog.Warnf("Invalid gas price %s", globals.FormatMoney(gasPrice))
		return nil, nil, 0, 0, fmt.Errorf("GasPrice is not enough")
	}
	if gasPrice == 0 {
		gasPrice = config.DEFAULT_GASPRICE
	}

	if gas < config.MIN_GASLIMIT {
		rlog.Warnf("Invalid gas price %s", globals.FormatMoney(gasPrice))
		return nil, nil, 0, 0, fmt.Errorf("Gas is not enough")
	}
	if gas == 0 {
		gas = config.DEFAULT_GASLIMIT
	}

	keys := w.Get_Keys()
	addr := w.GetAddress()

	txExtra := new(transaction.TxCreateExtra)
	txExtra.ContractData = &transaction.SCData{
		Sender:       addr.ToContractAddress(),
		AccountNonce: 0,
		Price:        gasPrice,
		GasLimit:     gas,
		Amount:       amount,
		Payload:      code,
		Sig:          crypto.Sign(code, keys.Spendkey_Secret),
	}

	if !isCreate {
		if contractAddr == "" {
			rlog.Warnf("Request param 'to' is empty")
			return nil, nil, 0, 0, fmt.Errorf("Need contract address")
		}
		to, err := address.NewAddress(contractAddr)
		if err != nil {
			rlog.Warnf("Request param 'to' is invalid")
			return nil, nil, 0, 0, fmt.Errorf("Contract address is invalid")
		}
		txExtra.ContractData.Recipient = to.ToContractAddress()
	}

	return w.TransferV2(nil, nil, 0, "", 0, 0, txExtra)
}

// build DEPOSIT and WITHDRAW transaction

func (w *Wallet) BuildDepositTx(amount uint64) (tx *transaction.Transaction, err error) {
	addr := w.GetAddress()
	txExtra := new(transaction.TxCreateExtra)
	txExtra.ContractData = &transaction.SCData{
		Sender:       addr.ToContractAddress(),
		Amount:       amount,
		Type:         transaction.SCDATA_DEPOSIT_TYPE,
	}

	var dests []address.Address
	var amounts []uint64
	dests = append(dests, *address.NewZeroAddress(globals.GetNetwork()))
	amounts = append(amounts, amount)

	tx,_,_,_,err = w.TransferV2(dests, amounts, 0, "", 0, 0, txExtra)

	return tx,err
}

func (w *Wallet) BuildWithdrawTx(uintAmount uint64) (tx *transaction.Transaction, err error) {
	addr := w.GetAddress()
	bytesAmount := uintAmountToBytesAmount(uintAmount)
	txExtra := new(transaction.TxCreateExtra)
	txExtra.ContractData = &transaction.SCData{
		Sender:       addr.ToContractAddress(),
		Type:         transaction.SCDATA_WITHDRAW_TYPE,
		Payload:      bytesAmount,
	}

	tx,_,_,_,err = w.TransferV2(nil, nil, 0, "", 0, 0, txExtra)

	return tx,err
}

func uintAmountToBytesAmount(amount uint64) []byte {
	buf := make([]byte,8) // sizeof(uint64) == 8 bytes
	binary.BigEndian.PutUint64(buf,amount)
	return buf
}