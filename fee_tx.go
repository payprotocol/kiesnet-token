// Copyright Key Inside Co., Ltd. 2019 All Rights Reserved.

package main

import (
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// Get fee list of token
// ISSUE : Shoud this be in token_tx.go? And should route name be token/fee/list?
// ISSUE : Should only target address holder be able to query fee list?
// ISSUE : Should we have to add “since_last_prune” parameter?
// params[0] : token code
// params[1] : optional. bookmark
// params[2] : optional. fetch size (if less than 1, default size. max 200)
// params[3] : optional. start time (timestamp represented by int64 seconds)
// params[4] : optional. end time (timestamp represented by in64 seconds)
func feeList(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	code, err := ValidateTokenCode(params[0])
	if nil != err {
		return shim.Error(err.Error())
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if nil != err {
		return responseError(err, "failed to get the token")
	}

	// authentication
	_, err = kid.GetID(stub, false)
	if nil != err {
		return shim.Error(err.Error())
	}

	bookmark := ""
	fetchSize := 0
	var stime, etime *txtime.Time
	// bookmark
	if len(params) > 1 {
		bookmark = params[1]
		// fetch size
		if len(params) > 2 {
			fetchSize, err = strconv.Atoi(params[2])
			if nil != err {
				return shim.Error("invalid fetch size")
			}
			// start time
			if len(params) > 3 {
				if len(params[3]) > 0 {
					seconds, err := strconv.ParseInt(params[3], 10, 64)
					if nil != err {
						return shim.Error("invalid start time: need seconds since 1970")
					}
					stime = txtime.Unix(seconds, 0)
				}
				// end time
				if len(params) > 4 {
					if len(params[4]) > 0 {
						seconds, err := strconv.ParseInt(params[4], 10, 64)
						if nil != err {
							return shim.Error("invalid end time: need seconds since 1970")
						}
						etime = txtime.Unix(seconds, 0)
						if nil != stime && stime.Cmp(etime) >= 0 {
							return shim.Error("invalid time parameters")
						}
					}
				}
			}
		}
	}

	fb := NewFeeStub(stub)
	res, err := fb.GetQueryFees(token.DOCTYPEID, bookmark, fetchSize, stime, etime)
	if nil != err {
		return responseError(err, "failed to get fees")
	}

	data, err := json.Marshal(res)
	if nil != err {
		return responseError(err, "failed to marshal fees")
	}

	return shim.Success(data)
}

// prune the fees from last fee time to end_time.
// if end_time is not provided, prune to 10 mins lesser than current time(if ten_minutes_flag is set to true).
// Only holder of FeePolicy.TargetAddress is able to prune.
// ISSUE : Shoud this be in token_tx.go? And should route name be token/fee/prune?
// params[0] : token code
// params[1] : 10 minutes limit flag. if the value is true, 10 minutes check is activated.
// params[2] : optional. end time
func feePrune(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 2 {
		return shim.Error("incorrect number of parameters. expecting 2+")
	}

	code, err := ValidateTokenCode(params[0])
	if nil != err {
		return shim.Error(err.Error())
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if nil != err {
		return responseError(err, "failed to get the token")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if nil != err {
		return shim.Error(err.Error())
	}

	// If Token.FeePolicy is nil, that means there is no fee utxo.
	if token.FeePolicy == nil {
		feeSum := &FeeSum{
			Sum:     &Amount{Int: *big.NewInt(0)},
			Count:   0,
			HasMore: false,
		}
		data, err := json.Marshal(feeSum)
		if nil != err {
			return responseError(err, "failed to marshal the fee prune result")
		}
		return shim.Success(data)
	}

	// If Token.FeePolicy is not nil, Token.FeePolicy.TargetAddress is never empty.
	// ISSUE : We MUST validate target address before(tokenUpdate)
	addr, _ := ParseAddress(token.FeePolicy.TargetAddress) // err is nil
	ab := NewAccountStub(stub, code)
	account, err := ab.GetAccount(addr)
	if nil != err {
		return responseError(err, "failed to get the target account")
	}
	if !account.HasHolder(kid) { // authority
		return shim.Error("no authority")
	}
	// ISSUE : What if target account is suspended?

	stime := txtime.Unix(0, 0)
	if len(token.LastPrunedFeeID) > 0 {
		//TODO check fee id parsing logic
		s, err := strconv.ParseInt(token.LastPrunedFeeID[0:10], 10, 64)
		if nil != err {
			return responseError(err, "failed to get seconds from timestamp")
		}
		//TODO check fee id parsing logic
		n, err := strconv.ParseInt(token.LastPrunedFeeID[10:19], 10, 64)
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
	if len(params) > 2 {
		seconds, err := strconv.ParseInt(params[2], 10, 64)
		if nil != err {
			return responseError(err, "failed to parse the end time")
		}
		etime = txtime.Unix(seconds, 0)
	} else {
		etime = ts
	}

	safely, err := strconv.ParseBool(params[1])
	if nil != err {
		return shim.Error("invalid boolean flag")
	}

	if safely {
		// safe time is current transaction time minus 10 minutes. this is to prevent missing pay(s) because of the time differences(+/- 5min) on different servers/devices
		safeTime := txtime.New(ts.Add(-6e+11))
		if nil == etime || etime.Cmp(safeTime) > 0 {
			etime = safeTime
		}
	}

	// calculate fee sum
	fb := NewFeeStub(stub)
	feeSum, err := fb.GetFeeSumByTime(code, stime, etime)
	if nil != err {
		return responseError(err, "failed to get fees to prune")
	}

	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(account.GetID())
	if nil != err {
		return responseError(err, "failed to get the target account balance")
	}

	if feeSum.Count > 0 {
		bal.Amount.Add(feeSum.Sum)
		bal.UpdatedTime = ts
		err = bb.PutBalance(bal)
		if nil != err {
			return responseError(err, "failed to update genesis account balance")
		}
		token.LastPrunedFeeID = feeSum.End
		err = tb.PutToken(token)
		if nil != err {
			return responseError(err, "failed to update the token")
		}
	}

	// balance log
	pruneLog := NewBalancePruneFeeLog(bal, *feeSum.Sum, feeSum.Start, feeSum.End)
	pruneLog.CreatedTime = ts
	err = bb.PutBalanceLog(pruneLog)
	if nil != err {
		return responseError(err, "failed to save balance log")
	}

	data, err := json.Marshal(feeSum)
	if nil != err {
		return responseError(err, "failed to marshal the fee prune result")
	}
	return shim.Success(data)
}
