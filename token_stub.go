// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// TokenStub _
type TokenStub struct {
	stub shim.ChaincodeStubInterface
}

// NewTokenStub _
func NewTokenStub(stub shim.ChaincodeStubInterface) *TokenStub {
	return &TokenStub{stub}
}

// CreateKey _
func (tb *TokenStub) CreateKey(code string) string {
	return "TKN_" + code
}

// CreateToken _
func (tb *TokenStub) CreateToken(code string, decimal int, maxSupply, supply Amount, feePolicy *FeePolicy, holders *stringset.Set) (*Token, error) {
	// create genesis account (joint account)
	ab := NewAccountStub(tb.stub, code)
	account, balance, err := ab.CreateJointAccount(holders)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the genesis account")
	}

	if feePolicy != nil {
		if len(feePolicy.TargetAddress) > 0 {
			if _, err := ab.GetAccountState(feePolicy.TargetAddress); err != nil {
				return nil, err
			}
		} else {
			feePolicy.TargetAddress = account.GetID()
		}
	}

	// initial mint
	if supply.Sign() > 0 {
		bb := NewBalanceStub(tb.stub)
		_, err = bb.Supply(balance, supply)
		if err != nil {
			return nil, errors.Wrap(err, "failed to mint initial supply")
		}
	}

	// token
	ts, err := txtime.GetTime(tb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}
	token := &Token{
		DOCTYPEID:      code,
		Decimal:        decimal,
		MaxSupply:      maxSupply,
		Supply:         supply,
		GenesisAccount: account.GetID(),
		FeePolicy:      feePolicy,
		CreatedTime:    ts,
		UpdatedTime:    ts,
	}
	if err = tb.PutToken(token); err != nil {
		return nil, errors.Wrap(err, "failed to create the token")
	}

	return token, nil
}

// GetToken _
func (tb *TokenStub) GetToken(code string) (*Token, error) {
	data, err := tb.GetTokenState(code)
	if err != nil {
		return nil, err
	}
	// data is not nil
	token := &Token{}
	if err = json.Unmarshal(data, token); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the token")
	}
	return token, nil
}

// GetTokenState _
func (tb *TokenStub) GetTokenState(code string) ([]byte, error) {
	data, err := tb.stub.GetState(tb.CreateKey(code))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the token state")
	}
	if data != nil {
		return data, nil
	}
	return nil, NotIssuedTokenError{code: code}
}

// PutToken _
func (tb *TokenStub) PutToken(token *Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the token")
	}
	if err = tb.stub.PutState(tb.CreateKey(token.DOCTYPEID), data); err != nil {
		return errors.Wrap(err, "failed to put the token state")
	}
	return nil
}

// Burn _
func (tb *TokenStub) Burn(token *Token, bal *Balance, amount Amount) (*Token, *BalanceLog, error) {
	ts, err := txtime.GetTime(tb.stub)
	if err != nil {
		return token, nil, errors.Wrap(err, "failed to get the timestamp")
	}

	// token
	if token.Supply.Sign() == 0 {
		return token, nil, SupplyError{reason: "no supply"}
	}

	// balance
	bb := NewBalanceStub(tb.stub)
	if bal.Amount.Cmp(&amount) < 0 {
		amount = *(bal.Amount.Copy()) // real diff
	}
	if amount.Sign() <= 0 { // nothing to burn
		return token, nil, nil
	}

	// burn
	amount.Neg() // -
	var log *BalanceLog
	if log, err = bb.Supply(bal, amount); err != nil {
		return token, log, errors.Wrap(err, "failed to burn")
	}

	// supply
	token.Supply.Add(&amount)
	token.UpdatedTime = ts
	if err = tb.PutToken(token); err != nil {
		return token, log, errors.Wrap(err, "failed to update the token")
	}

	return token, log, nil
}

// Mint _
func (tb *TokenStub) Mint(token *Token, bal *Balance, amount Amount) (*Token, *BalanceLog, error) {
	ts, err := txtime.GetTime(tb.stub)
	if err != nil {
		return token, nil, errors.Wrap(err, "failed to get the timestamp")
	}

	// token
	if token.Supply.Cmp(&token.MaxSupply) >= 0 {
		return token, nil, SupplyError{reason: "max supplied"}
	}

	// supply
	token.Supply.Add(&amount)
	if token.MaxSupply.Cmp(&token.Supply) < 0 {
		amount.Add(&token.MaxSupply)
		amount.Add(token.Supply.Neg()) // real diff
		token.Supply = token.MaxSupply
	}
	token.UpdatedTime = ts
	if err = tb.PutToken(token); err != nil {
		return token, nil, errors.Wrap(err, "failed to update the token")
	}

	// balance
	bb := NewBalanceStub(tb.stub)
	// mint
	var log *BalanceLog
	if log, err = bb.Supply(bal, amount); err != nil {
		return token, log, errors.Wrap(err, "failed to mint")
	}

	return token, log, nil
}
