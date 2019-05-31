// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/contract"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// params[0] : sender's address or token code
// params[1] : receiver's address
// params[2] : amount(>0)
// params[3] : optional. order id
// params[4] : optional. memo (see MemoMaxLength)
// params[5] : optional. expiry (duration represented by int64 seconds, multi-sig only)
func pay(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 3 {
		return shim.Error("incorrect number of parameters. expecting 3+")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if nil != err {
		return shim.Error(err.Error())
	}

	// addresses
	rAddr, err := ParseAddress(params[1])
	if err != nil {
		return responseError(err, "failed to parse the receiver's account address")
	}
	var sAddr *Address
	if len(params[0]) > 0 {
		sAddr, err = ParseAddress(params[0])
		if err != nil {
			return responseError(err, "failed to parse the sender's account address")
		}
		if rAddr.Code != sAddr.Code { // not same token
			return shim.Error("different token accounts")
		}
	} else {
		sAddr = NewAddress(rAddr.Code, AccountTypePersonal, kid)
	}

	// prevent from paying to self
	if sAddr.Equal(rAddr) {
		return shim.Error("can't pay to self")
	}

	// amount
	amount, err := NewAmount(params[2])
	if nil != err {
		return shim.Error(err.Error())
	}
	if amount.Sign() < 1 {
		return shim.Error("invalid amount. must be greater than 0")
	}

	ab := NewAccountStub(stub, rAddr.Code)

	// sender account validation
	sender, err := ab.GetAccount(sAddr)
	if nil != err {
		return responseError(err, "failed to get the sender account")
	}
	if !sender.HasHolder(kid) {
		return shim.Error("invoker is not holder")
	}
	if sender.IsSuspended() {
		return shim.Error("the sender account is suspended")
	}

	// receiver account validation
	receiver, err := ab.GetAccount(rAddr)
	if nil != err {
		return responseError(err, "failed to get the receiver account")
	}
	if receiver.IsSuspended() {
		return shim.Error("the receiver account is suspended")
	}

	// sender balance
	bb := NewBalanceStub(stub)
	sBal, err := bb.GetBalance(sender.GetID())
	if nil != err {
		return responseError(err, "failed to get the sender's balance")
	}

	if sBal.Amount.Cmp(amount) < 0 {
		return shim.Error("not enough balance")
	}

	// options
	memo := ""
	var expiry int64
	orderID := ""
	signers := stringset.New(kid)
	if a, ok := sender.(*JointAccount); ok {
		signers.AppendSet(a.Holders)
	}
	// order id
	if len(params) > 3 {
		orderID = params[3]
		// memo
		if len(params) > 4 {
			if len(params[4]) > MemoMaxLength { // length limit
				memo = params[4][:MemoMaxLength]
			} else {
				memo = params[4]
			}
			// expiry time
			if len(params) > 5 && len(params[5]) > 0 {
				expiry, err = strconv.ParseInt(params[5], 10, 64)
				if err != nil {
					responseError(err, "invalid expiry: need seconds")
				}
			}
		}
	}

	var log *BalanceLog // log for response
	payResult := &PayResult{}
	if signers.Size() > 1 {
		if signers.Size() > 128 {
			return shim.Error("too many signers")
		}
		// pending balance id
		pbID := stub.GetTxID()
		// contract
		doc := []string{"pay", pbID, sender.GetID(), receiver.GetID(), amount.String(), orderID, memo}
		docb, err := json.Marshal(doc)
		if err != nil {
			return responseError(err, "failed to marshal contract document")
		}
		con, err := contract.CreateContract(stub, docb, expiry, signers)
		if err != nil {
			return responseError(err, "failed to create a contract")
		}
		// pending balance
		// Cannot calculate fee amount now.
		// Fee amount must be calculated when the contract gets all of its approval.
		log, err = bb.Deposit(pbID, sBal, con, *amount, nil, memo)
		if err != nil {
			return responseError(err, "failed to create the pending balance")
		}
	} else {
		fb := NewFeeStub(stub)
		feeAmount, err := fb.CalcFee(rAddr, "pay", *amount)
		if err != nil {
			return responseError(err, "failed to get the fee amount")
		}
		payResult, err = NewPayStub(stub).Pay(sBal, receiver.GetID(), *amount, *feeAmount, orderID, memo)
		if err != nil {
			return responseError(err, "failed to pay")
		}
	}

	if payResult.BalanceLog == nil {
		payResult.BalanceLog = log
	}

	data, err := json.Marshal(payResult)
	if nil != err {
		return responseError(err, "failed to marshal the log")
	}

	return shim.Success(data)
}

// params[0] : original pay id
// params[1] : refund amount
// params[2] : optional. memo (see MemoMaxLength)
func payRefund(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 2 {
		return shim.Error("incorrect number of parameters. expecting 2+")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if nil != err {
		return shim.Error(err.Error())
	}

	// amount
	amount, err := NewAmount(params[1])
	if nil != err {
		return shim.Error(err.Error())
	}
	if amount.Sign() < 1 {
		return shim.Error("invalid amount. must be greater than 0")
	}

	pb := NewPayStub(stub)
	parentID := params[0]

	parentPay, err := pb.GetPay(parentID)
	if err != nil {
		return responseError(err, "failed to get the original payment")
	}

	// get sender from original pay
	var sAddr *Address
	sAddr, err = ParseAddress(parentPay.DOCTYPEID)
	if err != nil {
		return responseError(err, "failed to get the account")
	}

	// receiver's id from the original pay
	rid := parentPay.RID

	// receiver address validation
	rAddr, err := ParseAddress(rid)
	if err != nil {
		return responseError(err, "failed to parse the receiver's account address")
	}

	if rAddr.Code != sAddr.Code {
		return shim.Error("different token accounts")
	}

	if sAddr.Equal(rAddr) {
		return shim.Error("can't refund to self")
	}

	// refund amount validation
	if parentPay.Amount.Cmp(parentPay.TotalRefund.Copy().Add(amount)) < 0 {
		return shim.Error("can't exceed the original pay amount")
	}

	ab := NewAccountStub(stub, rAddr.Code)

	// sender account validation
	sender, err := ab.GetAccount(sAddr)
	if nil != err {
		return responseError(err, "failed to get the sender account")
	}
	if !sender.HasHolder(kid) {
		return shim.Error("invoker is not holder")
	}
	if sender.IsSuspended() {
		return shim.Error("the sender account is suspended")
	}

	// receiver account validation
	receiver, err := ab.GetAccount(rAddr)
	if nil != err {
		return responseError(err, "failed to get the receiver account")
	}
	if receiver.IsSuspended() {
		return shim.Error("the receiver account is suspended")
	}

	// sender balance
	bb := NewBalanceStub(stub)
	sBal, err := bb.GetBalance(sender.GetID())
	if nil != err {
		return responseError(err, "failed to get the sender's balance")
	}

	// receiver balance
	rBal, err := bb.GetBalance(receiver.GetID())
	if nil != err {
		return responseError(err, "failed to get the receiver's balance")
	}

	// options
	memo := ""
	// memo
	if len(params) > 2 {
		if len(params[2]) > MemoMaxLength { // length limit
			memo = params[2][:MemoMaxLength]
		} else {
			memo = params[2]
		}
	}

	// fee refund
	var feeAmount *Amount
	if amount.Cmp(&parentPay.Amount) != 0 { // partial refund
		// Apply rate at the time of payment.
		// Some of total fee amount(which the merchant could receive) may be lost
		// because below logic discards the precision, but it doesn't matter.
		// feeAmount = amount * parentPay.Fee / parentPay.Amount
		rat := new(big.Rat).SetFrac(&parentPay.Fee.Int, &parentPay.Amount.Int)
		feeAmount = amount.Copy().MulRat(rat)
	} else { // total refund
		feeAmount = parentPay.Fee.Copy()
	}

	var log *BalanceLog
	log, err = pb.Refund(sBal, rBal, *amount, *feeAmount, memo, parentPay)
	if err != nil {
		return responseError(err, "failed to pay")
	}

	// log is not nil
	data, err := json.Marshal(log)
	if nil != err {
		return responseError(err, "failed to marshal the log")
	}

	return shim.Success(data)
}

// params[0] : address to prune or token code
// params[1] : 10 minutes limit flag. if the value is true, 10 minutes check is activated.
// params[2] : optional. end time
func payPrune(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 2 {
		return shim.Error("incorrect number of parameters. expecting 2+")
	}
	// authentication
	kid, err := kid.GetID(stub, true)
	if nil != err {
		return shim.Error(err.Error())
	}

	var addr *Address
	code, err := ValidateTokenCode(params[0])
	if nil == err { // by token code
		addr = NewAddress(code, AccountTypePersonal, kid)
	} else { // by address
		addr, err = ParseAddress(params[0])
		if nil != err {
			return responseError(err, "failed to get the account")
		}
	}
	// account validation
	account, err := NewAccountStub(stub, addr.Code).GetAccount(addr)
	if nil != err {
		return responseError(err, "failed to get the account")
	}
	if !account.HasHolder(kid) {
		return shim.Error("invoker is not holder")
	}
	if account.IsSuspended() {
		return shim.Error("the account is suspended")
	}

	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(account.GetID())
	if nil != err {
		return responseError(err, "failed to get the balance")
	}
	pb := NewPayStub(stub)

	// start time
	stime := txtime.Unix(0, 0)
	if 0 < len(bal.LastPrunedPayID) {
		s, err := strconv.ParseInt(bal.LastPrunedPayID[0:10], 10, 64)
		if nil != err {
			return responseError(err, "failed to get seconds from timestamp")
		}
		n, err := strconv.ParseInt(bal.LastPrunedPayID[10:19], 10, 64)
		if nil != err {
			return responseError(err, "failed to get nanoseconds from timestamp")
		}
		stime = txtime.Unix(s, n)
	}

	ts, err := txtime.GetTime(stub)
	if nil != err {
		return responseError(err, "failed to get the timestamp")
	}

	var etime *txtime.Time
	// end time
	if len(params) > 2 {
		seconds, err := strconv.ParseInt(params[2], 10, 64)
		if nil != err {
			return responseError(err, "failed to parse the end time")
		}
		etime = txtime.Unix(seconds, 0)
	} else {
		etime = ts
	}

	//boolean validation
	b, err := strconv.ParseBool(params[1])
	if err != nil {
		return shim.Error("wrong first params value. the value must be true or false")
	}

	if b == true {
		// safe time is current transaction time minus 10 minutes. this is to prevent missing pay(s) because of the time differences(+/- 5min) on different servers/devices
		safeTime := txtime.New(ts.Add(-6e+11))
		if nil == etime || etime.Cmp(safeTime) > 0 {
			etime = safeTime
		}
	}

	paySum, err := pb.GetPaySumByTime(account.GetID(), stime, etime)
	if nil != err {
		return responseError(err, "failed to get pay(s) to prune")
	}
	// sum - fee
	applied := paySum.Sum.Copy().Add(paySum.Fee.Copy().Neg())

	// Add balance
	bal.Amount.Add(applied)
	bal.UpdatedTime = ts
	if 0 != len(paySum.End) {
		bal.LastPrunedPayID = paySum.End
	}

	if err := bb.PutBalance(bal); nil != err {
		return responseError(err, "failed to update balance")
	}

	if _, err = NewFeeStub(stub).CreateFee(account.GetID(), *paySum.Fee); err != nil {
		return shim.Error(err.Error())
	}

	// balance log
	rbl := NewBalancePrunePayLog(bal, *applied, paySum.Start, paySum.End)
	rbl.CreatedTime = ts
	if err = bb.PutBalanceLog(rbl); err != nil {
		return shim.Error(err.Error())
	}

	data, err := json.Marshal(paySum)
	if nil != err {
		return responseError(err, "failed to marshal the pay prune result")
	}
	return shim.Success(data)
}

// params[0] : token code | account address
// params[1] : sort order ("asc" or "desc")
// params[2] : bookmark
// params[3] : fetch size (if < 1 => default size, max 200)
// params[4] : start time (time represented by int64 seconds)
// params[5] : end time (time represented by int64 seconds)
func payList(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	// authentication
	kid, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	bookmark := ""
	fetchSize := 0
	sortOrder := "desc" // if not specified to "asc", default is decending order
	var stime, etime *txtime.Time
	// sort order
	if len(params) > 1 {
		if strings.ToLower(params[1]) == "asc" {
			sortOrder = "asc"
		}
		// bookmark
		if len(params) > 2 {
			bookmark = params[2]
			// fetch size
			if len(params) > 3 {
				fetchSize, err = strconv.Atoi(params[3])
				if err != nil {
					return responseError(err, "invalid fetch size")
				}
				// start time
				if len(params) > 4 {
					if len(params[4]) > 0 {
						seconds, err := strconv.ParseInt(params[4], 10, 64)
						if err != nil {
							return shim.Error("invalid start time: need seconds since 1970")
						}
						stime = txtime.Unix(seconds, 0)
					}
					// end time
					if len(params) > 5 {
						if len(params[5]) > 0 {
							seconds, err := strconv.ParseInt(params[5], 10, 64)
							if err != nil {
								return shim.Error("invalid end time: need seconds since 1970")
							}
							etime = txtime.Unix(seconds, 0)
							if stime != nil && stime.Cmp(etime) >= 0 {
								return shim.Error("invalid time parameters")
							}
						}
					}
				}
			}
		}
	}
	var addr *Address
	code, err := ValidateTokenCode(params[0])
	if nil == err { // by token code
		addr = NewAddress(code, AccountTypePersonal, kid)
	} else { // by address
		addr, err = ParseAddress(params[0])
		if err != nil {
			return responseError(err, "failed to parse the account address")
		}
	}

	res, err := NewPayStub(stub).GetPaysByTime(addr.String(), sortOrder, bookmark, stime, etime, fetchSize)
	if nil != err {
		return responseError(err, "failed to get pays log")
	}

	data, err := json.Marshal(res)
	if err != nil {
		return responseError(err, "failed to marshal pays logs")
	}

	return shim.Success(data)
}

// params[0] : pay id
// params[1] : optional. order id (vendor specific)
func payGet(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1 or 2")
	}

	// authentication
	_, err := kid.GetID(stub, false)
	if nil != err {
		return shim.Error(err.Error())
	}

	payID := params[0]
	orderID := ""
	if len(params) == 2 {
		orderID = params[1]
	}

	pb := NewPayStub(stub)
	var pay *Pay
	if "" == payID {
		if "" == orderID {
			return shim.Error("invalid parameter")
		}
		// get by order id
		pay, err = pb.GetPayByOrderID(orderID)
	} else {
		// get by pay id
		pay, err = pb.GetPay(payID)
	}
	if nil != err {
		return responseError(err, "failed to get pay")
	}
	data, err := json.Marshal(pay)
	if nil != err {
		return responseError(err, "failed to marshal the pay")
	}
	return shim.Success(data)
}

// contract callbacks

// doc: ["pay", pending-balance-ID, sender-ID, receiver-ID, amount, order-ID, memo]
func executePay(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) < 7 {
		return shim.Error("invalid contract document")
	}

	// pending balance
	bb := NewBalanceStub(stub)
	pb, err := bb.GetPendingBalance(doc[1].(string))
	if err != nil {
		return responseError(err, "failed to get the pending balance")
	}
	// validate
	if pb.Type != PendingBalanceTypeContract || pb.RID != cid {
		return shim.Error("invalid pending balance")
	}

	fb := NewFeeStub(stub)
	rAddr, _ := ParseAddress(doc[3].(string)) // merchant
	feeAmount, err := fb.CalcFee(rAddr, "pay", pb.Amount)
	if err != nil {
		return responseError(err, "failed to get the fee amount")
	}
	// ISSUE: check accounts ? (suspended) Business...
	if err = NewPayStub(stub).PayPendingBalance(pb, *feeAmount, doc[3].(string), doc[5].(string), doc[6].(string)); err != nil {
		return responseError(err, "failed to pay a pending balance")
	}

	return shim.Success(nil)
}
