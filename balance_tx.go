// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// params[0] : token code | account address
// params[1] : balance log type
// params[2] : bookmark
// params[3] : fetch size (if < 1 => default size, max 200)
// params[4] : start time (time represented by int64 seconds)
// params[5] : end time (time represented by int64 seconds)
func balanceLogs(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	// authentication
	kid, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	typeStr := ""
	bookmark := ""
	fetchSize := 0
	var stime, etime *txtime.Time
	// balance log type
	if len(params) > 1 {
		typeStr = params[1]
		// bookmark
		if len(params) > 2 {
			bookmark = params[2]
			// fetch size
			if len(params) > 3 {
				fetchSize, err = strconv.Atoi(params[3])
				if err != nil {
					return shim.Error("invalid fetch size")
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

	if typeStr != "" {
		if _, err := strconv.ParseInt(typeStr, 10, 8); nil != err {
			return responseError(err, "failed to parse balance log type")
		}
	}

	bb := NewBalanceStub(stub)
	res, err := bb.GetQueryBalanceLogs(addr.String(), typeStr, bookmark, fetchSize, stime, etime)
	if err != nil {
		return responseError(err, "failed to get balance logs")
	}

	data, err := json.Marshal(res)
	if err != nil {
		return responseError(err, "failed to marshal balance logs")
	}
	return shim.Success(data)
}

// params[0] : pending balance id
func balancePendingGet(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	// authentication
	_, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	// pending balance
	bb := NewBalanceStub(stub)
	pb, err := bb.GetPendingBalance(params[0])
	if err != nil {
		return responseError(err, "failed to get the pending balance")
	}

	data, err := json.Marshal(pb)
	if err != nil {
		return responseError(err, "failed to marshal the pending balance")
	}

	return shim.Success(data)
}

// params[0] : token code | account address
// params[1] : sort ('created_time' | 'pending_time')
// params[2] : bookmark
// params[3] : fetch size (if < 1 => default size, max 200)
func balancePendingList(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	// authentication
	kid, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	sort := "pending_time"
	bookmark := ""
	fetchSize := 0
	// sort
	if len(params) > 1 {
		sort = params[1]
		// bookmark
		if len(params) > 2 {
			bookmark = params[2]
			// fetch size
			if len(params) > 3 {
				fetchSize, err = strconv.Atoi(params[3])
				if err != nil {
					return shim.Error("invalid fetch size")
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

	bb := NewBalanceStub(stub)
	res, err := bb.GetQueryPendingBalances(addr.String(), sort, bookmark, fetchSize)
	if err != nil {
		return responseError(err, "failed to get pending balances")
	}

	data, err := json.Marshal(res)
	if err != nil {
		return responseError(err, "failed to marshal pending balances")
	}
	return shim.Success(data)
}

// params[0] : pending balance id
func balancePendingWithdraw(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	ts, err := txtime.GetTime(stub)
	if err != nil {
		return responseError(err, "failed to get the timestamp")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	// pending balance
	bb := NewBalanceStub(stub)
	pb, err := bb.GetPendingBalance(params[0])
	if err != nil {
		return responseError(err, "failed to get the pending balance")
	}
	if pb.PendingTime.Cmp(ts) > 0 {
		return shim.Error("too early to withdraw")
	}

	// account
	addr, _ := ParseAddress(pb.Account) // err is nil
	ab := NewAccountStub(stub, addr.Code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to get the account")
	}
	if !account.HasHolder(kid) {
		return shim.Error("invoker is not holder")
	}
	if account.IsSuspended() {
		return shim.Error("the account is suspended")
	}

	// withdraw
	log, err := bb.Withdraw(pb)
	if err != nil {
		return responseError(err, "failed to withdraw")
	}

	data, err := json.Marshal(log)
	if err != nil {
		return responseError(err, "failed to marshal the log")
	}

	return shim.Success(data)
}
