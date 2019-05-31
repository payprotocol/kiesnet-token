// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/ccid"
	"github.com/key-inside/kiesnet-ccpkg/contract"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
)

// CtrFunc _
type CtrFunc func(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response

// routes is the map of contract functions
var ctrRoutes = map[string][]CtrFunc{
	"account/create":        []CtrFunc{contractVoid, executeAccountCreate},
	"account/holder/add":    []CtrFunc{contractVoid, executeAccountHolderAdd},
	"account/holder/remove": []CtrFunc{contractVoid, executeAccountHolderRemove},
	"pay":                   []CtrFunc{cancelTransfer, executePay},
	"token/burn":            []CtrFunc{contractVoid, executeTokenBurn},
	"token/create":          []CtrFunc{contractVoid, executeTokenCreate},
	"token/mint":            []CtrFunc{contractVoid, executeTokenMint},
	"transfer":              []CtrFunc{cancelTransfer, executeTransfer},
}

// fnIdx : 0 = cancel, 1 = execute
// params[0] : contract ID
// params[1] : contract document
func contractCallback(stub shim.ChaincodeStubInterface, fnIdx int, params []string) peer.Response {
	if len(params) != 2 {
		return shim.Error("incorrect number of parameters. expecting 2")
	}

	ccid, err := ccid.GetID(stub)
	if err != nil {
		return shim.Error("failed to get ccid")
	}
	if "kiesnet-contract" != ccid && "kiesnet-cc-contract" != ccid {
		return shim.Error("invalid access")
	}

	// authentication
	_, err = kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	cid := params[0] // contract ID
	doc := []interface{}{}
	err = json.Unmarshal([]byte(params[1]), &doc)
	if err != nil {
		return responseError(err, "failed to unmarshal the contract document")
	}
	dtype := doc[0].(string)
	if ctrFn := ctrRoutes[dtype][fnIdx]; ctrFn != nil {
		return ctrFn(stub, cid, doc)
	}
	return shim.Error("unknown contract: [" + dtype + "]")
}

func contractCancel(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	return contractCallback(stub, 0, params)
}

func contractExecute(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	return contractCallback(stub, 1, params)
}

// callback has nothing to do
func contractVoid(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	return shim.Success(nil)
}

// helpers

func invokeContract(stub shim.ChaincodeStubInterface, doc []interface{}, signers *stringset.Set) peer.Response {
	docb, err := json.Marshal(doc)
	if err != nil {
		return responseError(err, "failed to marshal the contract document")
	}
	con, err := contract.CreateContract(stub, docb, 0, signers)
	if err != nil {
		return responseError(err, "failed to create a contract")
	}
	data, err := con.MarshalJSON()
	if err != nil {
		return responseError(err, "failed to marshal the contract")
	}
	return shim.Success(data)
}
