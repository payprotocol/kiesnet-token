// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/pkg/errors"
)

// params[0] : token code
// params[1:] : co-holders' personal account addresses (exclude invoker, max 127)
func accountCreate(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// validate available token
	tb := NewTokenStub(stub)
	if _, err = tb.GetTokenState(code); err != nil { // check issued
		if _, ok := err.(NotIssuedTokenError); !ok {
			return responseError(err, "failed to create an account")
		}
		// check knt chaincode
		if _, err = invokeKNT(stub, code, []string{"token"}); err != nil {
			logger.Debug(err.Error())
			return shim.Error("failed to get the token meta")
		}
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	ab := NewAccountStub(stub, code)

	if len(params) < 2 { // personal account
		account, balance, err := ab.CreateAccount(kid)
		if err != nil {
			return responseError(err, "failed to create a personal account")
		}
		return responseAccountWithBalance(account, balance)
	}

	// joint account

	// check invoker's main(personal) account
	if _, err = ab.GetAccount(NewAddress(code, AccountTypePersonal, kid)); err != nil {
		return responseError(err, "failed to get invoker's personal account")
	}

	holders := stringset.New(kid) // KIDs

	addrs := stringset.New(params[1:]...) // remove duplication
	if addrs.Size() > 128 {
		return shim.Error("too many holders")
	}
	// validate & get kid of co-holders
	for addr := range addrs.Map() {
		kids, err := ab.GetSignableIDs(addr)
		if err != nil {
			return responseError(err, "invalid co-holder")
		}
		holders.AppendSlice(kids)
	}

	if holders.Size() < 2 { // addrs had invoker's addr
		return shim.Error("joint account needs co-holders")
	}

	// contract
	doc := []interface{}{"account/create", code, holders.Strings()}
	return invokeContract(stub, doc, holders)
}

// information of the account
// params[0] : token code | account address
func accountGet(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	// authentication
	kid, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	var addr *Address
	code, err := ValidateTokenCode(params[0])
	if nil == err { // by token code
		addr = NewAddress(code, AccountTypePersonal, kid)
	} else { // by address
		addr, err = ParseAddress(params[0])
		if err != nil {
			return responseError(err, "failed to get the account")
		}
	}
	ab := NewAccountStub(stub, addr.Code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to get the account")
	}

	// balance state
	bb := NewBalanceStub(stub)
	balance, err := bb.GetBalanceState(account.GetID())
	if err != nil {
		return responseError(err, "failed to get the account balance")
	}

	return responseAccountWithBalanceState(account, balance)
}

// params[0] : account address (joint account only)
// params[1] : co-holder's personal account address
func accountHolderAdd(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	jac, taddr, err := getValidatedAccountHolderParameters(stub, params)
	if err != nil {
		return shim.Error(err.Error())
	}
	if jac.Holders.Size() > 127 {
		return shim.Error("already has max holders (128)")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	if !jac.HasHolder(kid) {
		return shim.Error("no authority")
	}

	ab := NewAccountStub(stub, "")
	account, err := ab.GetAccount(taddr)
	if err != nil {
		return responseError(err, "failed to get the holder")
	}
	pac := account.(*Account)
	holder := pac.Holder()

	if jac.HasHolder(holder) {
		return shim.Error("existed holder")
	}

	signers := stringset.New(holder)
	signers.AppendSet(jac.Holders)

	// contract
	doc := []interface{}{"account/holder/add", jac.GetID(), holder}
	return invokeContract(stub, doc, signers)
}

// params[0] : account address (joint account only)
// params[1] : co-holder's personal account address
func accountHolderRemove(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	jac, taddr, err := getValidatedAccountHolderParameters(stub, params)
	if err != nil {
		return shim.Error(err.Error())
	}
	if jac.Holders.Size() < 3 {
		return shim.Error("the account has minimum holders (2)")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	if !jac.HasHolder(kid) {
		return shim.Error("no authority")
	}

	holder := taddr.ID()
	if holder == kid { // self out
		ab := NewAccountStub(stub, "")
		jac, err = ab.RemoveHolder(jac, kid)
		if err != nil {
			return responseError(err, "failed to remove the holder")
		}
		// balance state
		bb := NewBalanceStub(stub)
		balance, err := bb.GetBalanceState(jac.GetID())
		if err != nil {
			return responseError(err, "failed to get updated account")
		}
		return responseAccountWithBalanceState(jac, balance)
	}

	if !jac.HasHolder(holder) {
		return shim.Error("not existed holder")
	}

	signers := stringset.New()
	signers.AppendSet(jac.Holders)
	signers.Remove(holder)

	// contract
	doc := []interface{}{"account/holder/remove", jac.GetID(), holder}
	return invokeContract(stub, doc, signers)
}

// list of account's addresses
// params[0] : "" | token code
// params[1] : bookmark
// params[2] : fetch size (if < 1 => default size, max 200)
// ISSUE: list by an account address (privacy problem)
func accountList(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	// authentication
	kid, err := kid.GetID(stub, false)
	if err != nil {
		return shim.Error(err.Error())
	}

	code := ""
	bookmark := ""
	fetchSize := 0
	// code
	if len(params) > 0 {
		if len(params[0]) > 0 {
			code, err = ValidateTokenCode(params[0])
			if err != nil {
				return shim.Error(err.Error())
			}
		}
		// bookamrk
		if len(params) > 1 {
			bookmark = params[1]
			// fetch size
			if len(params) > 2 {
				fetchSize, err = strconv.Atoi(params[2])
				if err != nil {
					return shim.Error("invalid fetch size")
				}
			}
		}
	}

	ab := NewAccountStub(stub, code)
	res, err := ab.GetQueryHolderAccounts(kid, bookmark, fetchSize)
	if err != nil {
		return responseError(err, "failed to get accounts list")
	}

	data, err := json.Marshal(res)
	if err != nil {
		return responseError(err, "failed to marshal accounts list")
	}
	return shim.Success(data)
}

// ISSUE: more complex suspend/unsuspend ? (ex, joint account, admin ...)
// suspend personal(main) account of the token
// params[0] : token code
func accountSuspend(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	ab := NewAccountStub(stub, code)
	account, err := ab.SuspendAccount(kid)
	if err != nil {
		return responseError(err, "failed to suspend the account")
	}

	data, err := json.Marshal(account)
	if err != nil {
		return responseError(err, "failed to marshal the account")
	}
	return shim.Success(data)
}

// unsuspend personal(main) account of the token
// params[0] : token code
func accountUnsuspend(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	ab := NewAccountStub(stub, code)
	account, err := ab.UnsuspendAccount(kid)
	if err != nil {
		return responseError(err, "failed to unsuspend the account")
	}

	data, err := json.Marshal(account)
	if err != nil {
		return responseError(err, "failed to marshal the account")
	}
	return shim.Success(data)
}

// helpers

func getValidatedAccountHolderParameters(stub shim.ChaincodeStubInterface, params []string) (*JointAccount, *Address, error) {
	if len(params) != 2 {
		return nil, nil, errors.New("incorrect number of parameters. expecting 2")
	}

	addr, err := ParseAddress(params[0])
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse the account address")
	}
	if addr.Type != AccountTypeJoint {
		return nil, nil, errors.New("the account must be joint account")
	}

	taddr, err := ParseAddress(params[1])
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse the co-holder's account address")
	}
	if taddr.Type != AccountTypePersonal {
		return nil, nil, errors.New("the co-holder's account must be personal account")
	}

	if addr.Code != taddr.Code {
		return nil, nil, errors.New("mismatched token accounts")
	}

	ab := NewAccountStub(stub, addr.Code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the account")
	}
	jac := account.(*JointAccount)

	return jac, taddr, nil
}

func responseAccountWithBalance(account AccountInterface, balance *Balance) peer.Response {
	var data []byte
	var err error
	if a, ok := account.(*JointAccount); ok {
		data, err = json.Marshal(&struct {
			*JointAccount
			Balance *Balance `json:"balance"`
		}{a, balance})
	} else if a, ok := account.(*Account); ok {
		data, err = json.Marshal(&struct {
			*Account
			Balance *Balance `json:"balance"`
		}{a, balance})
	} else { // never here
		return shim.Error("unknown account type")
	}
	if err != nil {
		return responseError(err, "failed to marshal the payload")
	}
	return shim.Success(data)
}

func responseAccountWithBalanceState(account AccountInterface, balance []byte) peer.Response {
	data, err := json.Marshal(account)
	if err != nil {
		return responseError(err, "failed to marshal the account")
	}
	buf := bytes.NewBuffer(data[:(len(data) - 1)]) // eliminate last '}'
	if _, err := buf.WriteString(`,"balance":`); nil == err {
		if _, err := buf.Write(balance); nil == err {
			if err := buf.WriteByte('}'); nil == err {
				return shim.Success(buf.Bytes())
			}
		}
	}
	return responseError(err, "failed to marshal the payload")
}

// contract callbacks

// doc: ["account/create", code, [co-holders...]]
func executeAccountCreate(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) < 3 {
		return shim.Error("invalid contract document")
	}

	code := doc[1].(string)
	kids := doc[2].([]interface{})
	holders := stringset.New()
	for _, kid := range kids {
		holders.Add(kid.(string))
	}

	ab := NewAccountStub(stub, code)
	if _, _, err := ab.CreateJointAccount(holders); err != nil {
		return responseError(err, "failed to create a joint account")
	}

	return shim.Success(nil)
}

// doc: ["account/holder/add", address, holder-kid]
func executeAccountHolderAdd(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) < 3 {
		return shim.Error("invalid contract document")
	}

	addr, err := ParseAddress(doc[1].(string))
	if err != nil {
		return responseError(err, "failed to add the holder")
	}
	holder := doc[2].(string)

	ab := NewAccountStub(stub, addr.Code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to add the holder")
	}
	jac := account.(*JointAccount)

	// validate
	if jac.Holders.Size() > 127 {
		return shim.Error("already has max holders (128)")
	}
	if jac.HasHolder(holder) {
		return shim.Error("existed holder")
	}

	if _, err = ab.AddHolder(jac, holder); err != nil {
		return responseError(err, "failed to add the holder")
	}

	return shim.Success(nil)
}

// doc: ["account/holder/remove", address, holder-kid]
func executeAccountHolderRemove(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) < 3 {
		return shim.Error("invalid contract document")
	}

	addr, err := ParseAddress(doc[1].(string))
	if err != nil {
		return responseError(err, "failed to remove the holder")
	}
	holder := doc[2].(string)

	ab := NewAccountStub(stub, addr.Code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to remove the holder")
	}
	jac := account.(*JointAccount)

	// validate
	if jac.Holders.Size() < 3 {
		return shim.Error("the account has minimum holders (2)")
	}
	if !jac.HasHolder(holder) {
		return shim.Error("not existed holder")
	}

	if _, err = ab.RemoveHolder(jac, holder); err != nil {
		return responseError(err, "failed to remove the holder")
	}

	return shim.Success(nil)
}
