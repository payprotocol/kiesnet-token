// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
	"github.com/key-inside/kiesnet-ccpkg/contract"
	"github.com/key-inside/kiesnet-ccpkg/kid"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// params[0] : token code
// params[1] : amount (big int string)
func tokenBurn(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 2 {
		return shim.Error("incorrect number of parameters. expecting 2")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if err != nil {
		return responseError(err, "failed to get the token")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	// genesis account
	addr, _ := ParseAddress(token.GenesisAccount) // err is nil
	ab := NewAccountStub(stub, code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to get the genesis account")
	}
	if !account.HasHolder(kid) { // authority
		return shim.Error("no authority")
	}

	// balance
	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(account.GetID())
	if err != nil {
		return responseError(err, "failed to get the genesis account balance")
	}
	if bal.Amount.Sign() == 0 {
		return shim.Error("genesis account balance is 0")
	}

	_amount, err := NewAmount(params[1]) // validate amount
	if err != nil {
		return shim.Error(err.Error())
	}
	// get burnable amount
	burnable, err := invokeKNT(stub, code, []string{"burn", token.Supply.String(), bal.Amount.String(), _amount.String()})
	if err != nil {
		return responseError(err, "failed to get the burnable amount")
	}
	amount, err := NewAmount(string(burnable))
	if err != nil || amount.Sign() < 0 {
		return shim.Error("not burnable")
	}

	if token.Supply.Cmp(amount) < 0 {
		return shim.Error("amount must be less or equal than total supply")
	}

	jac := account.(*JointAccount)
	if jac.Holders.Size() > 1 {
		// contract
		doc := []interface{}{"token/burn", code, amount.String()}
		docb, err := json.Marshal(doc)
		if err != nil {
			logger.Debug(err.Error())
			return shim.Error("failed to create a contract")
		}
		// ISSUE : should we get and set expiry?
		con, err := contract.CreateContract(stub, docb, 0, jac.Holders)
		if err != nil {
			return shim.Error(err.Error())
		}
		payload := &TokenResult{Contract: con}
		data, err := json.Marshal(payload)
		if err != nil {
			return responseError(err, "failed to marshal payload")
		}
		return shim.Success(data)
	}

	// burn
	token, log, err := tb.Burn(token, bal, *amount)
	if err != nil {
		return responseError(err, "failed to burn")
	}

	payload := &TokenResult{Token: token, BalanceLog: log}
	data, err := json.Marshal(payload)
	if err != nil {
		return responseError(err, "failed to marshal payload")
	}
	return shim.Success(data)
}

// params[0] : token code (3~6 alphanum)
// params[1:] : co-holders (personal account addresses)
func tokenCreate(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) < 1 {
		return shim.Error("incorrect number of parameters. expecting 1+")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	tb := NewTokenStub(stub)
	if _, err = tb.GetTokenState(code); err != nil { // check issued
		if _, ok := err.(NotIssuedTokenError); !ok {
			return responseError(err, "failed to get the token state")
		}
	} else {
		return shim.Error("already issued token : [" + code + "]")
	}

	decimal, maxSupply, supply, feePolicy, err := getValidatedTokenMeta(stub, code)
	if err != nil {
		return shim.Error(err.Error())
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	// co-holders
	holders := stringset.New(kid)
	if len(params) > 1 {
		ab := NewAccountStub(stub, code)
		addrs := stringset.New(params[1:]...) // remove duplication
		// validate co-holders
		for addr := range addrs.Map() {
			kids, err := ab.GetSignableIDs(addr)
			if err != nil {
				return responseError(err, "invalid co-holder")
			}
			holders.AppendSlice(kids)
		}
	}

	if holders.Size() > 1 {
		// contract
		doc := []interface{}{"token/create", code, holders.Strings()}
		return invokeContract(stub, doc, holders)
	}

	token, err := tb.CreateToken(code, decimal, *maxSupply, *supply, feePolicy, holders)
	if err != nil {
		return responseError(err, "failed to create the token")
	}

	data, err := json.Marshal(token)
	if err != nil {
		return responseError(err, "failed to marshal the token")
	}
	return shim.Success(data)
}

// params[0] : token code
func tokenGet(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// authentication
	if _, err = kid.GetID(stub, false); err != nil {
		return shim.Error(err.Error())
	}

	tb := NewTokenStub(stub)
	data, err := tb.GetTokenState(code)
	if err != nil {
		return responseError(err, "failed to get the token state")
	}
	return shim.Success(data)
}

// params[0] : token code
// params[1] : amount (big int string)
func tokenMint(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 2 {
		return shim.Error("incorrect number of parameters. expecting 2")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if err != nil {
		return responseError(err, "failed to get the token")
	}
	if token.Supply.Cmp(&token.MaxSupply) >= 0 {
		return shim.Error("max supplied")
	}

	// authentication
	kid, err := kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	// genesis account
	addr, _ := ParseAddress(token.GenesisAccount) // err is nil
	ab := NewAccountStub(stub, code)
	account, err := ab.GetAccount(addr)
	if err != nil {
		return responseError(err, "failed to get the genesis account")
	}
	if !account.HasHolder(kid) { // authority
		return shim.Error("no authority")
	}

	// balance
	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(account.GetID())
	if err != nil {
		return responseError(err, "failed to get the genesis account balance")
	}

	_amount, err := NewAmount(params[1]) // validate amount
	if err != nil {
		return shim.Error(err.Error())
	}
	// get mintable amount
	mintable, err := invokeKNT(stub, code, []string{"mint", token.Supply.String(), bal.Amount.String(), _amount.String()})
	if err != nil {
		return responseError(err, "failed to get the mintable amount")
	}
	amount, err := NewAmount(string(mintable))
	if err != nil || amount.Sign() < 0 {
		return shim.Error("not mintable")
	}

	jac := account.(*JointAccount)
	if jac.Holders.Size() > 1 {
		// contract
		doc := []interface{}{"token/mint", code, amount.String()}
		// return invokeContract(stub, doc, jac.Holders)
		docb, err := json.Marshal(doc)
		if err != nil {
			logger.Debug(err.Error())
			return shim.Error("failed to create a contract")
		}
		// ISSUE : should we get and set expiry?
		con, err := contract.CreateContract(stub, docb, 0, jac.Holders)
		if err != nil {
			return shim.Error(err.Error())
		}
		payload := &TokenResult{Contract: con}
		data, err := json.Marshal(payload)
		if err != nil {
			return responseError(err, "failed to marshal payload")
		}
		return shim.Success(data)
	}

	// mint
	token, log, err := tb.Mint(token, bal, *amount)
	if err != nil {
		return responseError(err, "failed to mint")
	}

	payload := &TokenResult{Token: token, BalanceLog: log}
	data, err := json.Marshal(payload)
	if err != nil {
		return responseError(err, "failed to marshal payload")
	}
	return shim.Success(data)
}

// Get updated information from the token meta chaincode(e.g. knt-cc-pci) and save it to the ledger.
// params[0] : token code
func tokenUpdate(stub shim.ChaincodeStubInterface, params []string) peer.Response {
	if len(params) != 1 {
		return shim.Error("incorrect number of parameters. expecting 1")
	}

	code, err := ValidateTokenCode(params[0])
	if err != nil {
		return shim.Error(err.Error())
	}

	// authentication
	// ISSUE: only genesis account holders ?
	_, err = kid.GetID(stub, true)
	if err != nil {
		return shim.Error(err.Error())
	}

	// check issued token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if err != nil {
		return responseError(err, "failed to get the token")
	}

	// get token meta
	_, _, _, policy, err := getValidatedTokenMeta(stub, code)
	if err != nil {
		return shim.Error(err.Error())
	}

	// Update token state.
	var update bool
	if policy == nil {
		// Ignore knt target address if knt fee is empty.
		if token.FeePolicy == nil {
			// knt fee is empty, also token.FeePolicy is nil
			// nothing happened.
			update = false
		} else {
			// knt fee is empty, but token.FeePolicy exists.
			// but we must not make Token.FeePolicy nil.
			// keep the target address
			// remove all rates
			token.FeePolicy.Rates = map[string]FeeRate{}
			update = true
		}
	} else {
		if len(policy.TargetAddress) > 0 {
			if _, err := NewAccountStub(stub, code).GetAccountState(policy.TargetAddress); err != nil {
				return responseError(err, "failed to set a target address")
			}
		} else { // No new target address input. Do not edit current value.
			if token.FeePolicy == nil {
				policy.TargetAddress = token.GenesisAccount
			} else {
				policy.TargetAddress = token.FeePolicy.TargetAddress
			}
		}
		token.FeePolicy = policy
		update = true
	}
	if update {
		ts, err := txtime.GetTime(stub)
		if err != nil {
			return responseError(err, "failed to get the timestamp")
		}
		token.UpdatedTime = ts
		err = tb.PutToken(token)
		if err != nil {
			return responseError(err, "failed to update the token")
		}
	}

	data, err := json.Marshal(token)
	if err != nil {
		return responseError(err, "failed to marshal the token")
	}
	return shim.Success(data)
}

// helpers

func invokeKNT(stub shim.ChaincodeStubInterface, code string, params []string) ([]byte, error) {
	ccid := strings.ToLower(code)
	if os.Getenv("DEV_CHANNEL_NAME") != "" {
		ccid = "knt-cc-" + ccid
	} else {
		ccid = "knt-" + ccid
	}
	args := [][]byte{}
	for _, p := range params {
		args = append(args, []byte(p))
	}
	res := stub.InvokeChaincode(ccid, args, "")
	if res.GetStatus() == 200 {
		return res.GetPayload(), nil
	}
	// ISSUE: arbitrary token
	return nil, errors.New(res.GetMessage())
}

func getValidatedTokenMeta(stub shim.ChaincodeStubInterface, code string) (int, *Amount, *Amount, *FeePolicy, error) {
	// get token meta
	meta, err := invokeKNT(stub, code, []string{"token"})
	if err != nil {
		return 0, nil, nil, nil, errors.Wrap(err, "failed to get the token meta")
	}
	metaMap := map[string]string{}
	if err = json.Unmarshal(meta, &metaMap); err != nil {
		return 0, nil, nil, nil, errors.Wrap(err, "failed to unmarshal the token meta")
	}

	// validate meta
	decimal, err := strconv.Atoi(metaMap["decimal"])
	if err != nil || decimal < 0 || decimal > 18 {
		return 0, nil, nil, nil, errors.New("decimal must be integer between 0 and 18")
	}
	maxSupply, err := NewAmount(metaMap["max_supply"])
	if err != nil || maxSupply.Sign() < 0 {
		return 0, nil, nil, nil, errors.New("max supply must be positive integer")
	}
	supply, err := NewAmount(metaMap["initial_supply"])
	if err != nil || supply.Sign() < 0 || supply.Cmp(maxSupply) > 0 {
		return 0, nil, nil, nil, errors.New("initial supply must be positive integer and less(or equal) than max supply")
	}
	fee := metaMap["fee"]
	var policy *FeePolicy
	if len(fee) > 0 {
		policy, err = ParseFeePolicy(fee)
		if err != nil {
			return 0, nil, nil, nil, err
		}
		policy.TargetAddress = metaMap["target_address"]
	}

	return decimal, maxSupply, supply, policy, nil
}

// contract callbacks

// doc: ["token/burn", code, amount]
func executeTokenBurn(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) != 3 {
		return shim.Error("invalid contract document")
	}

	code := doc[1].(string)
	amount, err := NewAmount(doc[2].(string))
	if err != nil {
		return shim.Error("invalid amount")
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if err != nil {
		return responseError(err, "failed to get the token")
	}

	// balance
	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(token.GenesisAccount)
	if err != nil {
		return responseError(err, "failed to get the genesis account balance")
	}

	if _, _, err = tb.Burn(token, bal, *amount); err != nil {
		return responseError(err, "failed to burn")
	}

	return shim.Success(nil)
}

// doc: ["token/create", code, [co-holders...]]
func executeTokenCreate(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) != 3 {
		return shim.Error("invalid contract document")
	}

	code := doc[1].(string)

	tb := NewTokenStub(stub)
	if _, err := tb.GetTokenState(code); err != nil { // check issued
		if _, ok := err.(NotIssuedTokenError); !ok {
			return responseError(err, "failed to get the token state")
		}
	} else {
		return shim.Error("already issued token : [" + code + "]")
	}

	decimal, maxSupply, supply, feePolicy, err := getValidatedTokenMeta(stub, code)
	if err != nil {
		return shim.Error(err.Error())
	}

	kids := doc[2].([]interface{})
	holders := stringset.New()
	for _, kid := range kids {
		holders.Add(kid.(string))
	}

	if _, err = tb.CreateToken(code, decimal, *maxSupply, *supply, feePolicy, holders); err != nil {
		return responseError(err, "failed to create the token")
	}

	return shim.Success(nil)
}

// doc: ["token/mint", code, amount]
func executeTokenMint(stub shim.ChaincodeStubInterface, cid string, doc []interface{}) peer.Response {
	if len(doc) != 3 {
		return shim.Error("invalid contract document")
	}

	code := doc[1].(string)
	amount, err := NewAmount(doc[2].(string))
	if err != nil {
		return shim.Error("invalid amount")
	}

	// token
	tb := NewTokenStub(stub)
	token, err := tb.GetToken(code)
	if err != nil {
		return responseError(err, "failed to get the token")
	}

	// balance
	bb := NewBalanceStub(stub)
	bal, err := bb.GetBalance(token.GenesisAccount)
	if err != nil {
		return responseError(err, "failed to get the genesis account balance")
	}

	if _, _, err = tb.Mint(token, bal, *amount); err != nil {
		return responseError(err, "failed to mint")
	}

	return shim.Success(nil)
}
