// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// AccountsFetchSize _
const AccountsFetchSize = 20

// AccountStub _
type AccountStub struct {
	stub  shim.ChaincodeStubInterface
	token string
}

// NewAccountStub _
func NewAccountStub(stub shim.ChaincodeStubInterface, tokenCode string) *AccountStub {
	return &AccountStub{
		stub:  stub,
		token: tokenCode,
	}
}

// CreateKey _
func (ab *AccountStub) CreateKey(id string) string {
	return "ACC_" + id
}

// CreateAccount _
func (ab *AccountStub) CreateAccount(kid string) (*Account, *Balance, error) {
	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the timestamp")
	}

	addr := NewAddress(ab.token, AccountTypePersonal, kid)
	_, err = ab.GetAccount(addr)
	if err != nil {
		if _, ok := err.(NotExistedAccountError); !ok {
			return nil, nil, errors.Wrap(err, "failed to get an existed account")
		}

		// create personal account
		account := &Account{
			DOCTYPEID:   addr.String(),
			Token:       ab.token,
			Type:        AccountTypePersonal,
			CreatedTime: ts,
			UpdatedTime: ts,
		}
		if err = ab.PutAccount(account); err != nil {
			return nil, nil, errors.Wrap(err, "failed to create an account")
		}

		// balance
		bb := NewBalanceStub(ab.stub)
		balance, err := bb.CreateBalance(account.GetID())
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to create an account balance")
		}

		// create account-holder relationship
		holder := NewHolder(kid, account)
		holder.CreatedTime = ts
		if err = ab.PutHolder(holder); err != nil {
			return nil, nil, errors.Wrap(err, "failed to create the holder")
		}

		return account, balance, nil
	}

	return nil, nil, ExistedAccountError{addr: addr.String()}
}

// CreateJointAccount _
func (ab *AccountStub) CreateJointAccount(holders *stringset.Set) (*JointAccount, *Balance, error) {
	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get the timestamp")
	}

	addr := NewAddress(ab.token, AccountTypeJoint, ab.stub.GetTxID()) // random address
	if _, err = ab.GetAccount(addr); err != nil {
		if _, ok := err.(NotExistedAccountError); !ok {
			return nil, nil, errors.Wrap(err, "failed to get an existed account")
		}

		// create joint account
		account := &JointAccount{
			Account: Account{
				DOCTYPEID:   addr.String(),
				Token:       ab.token,
				Type:        AccountTypeJoint,
				CreatedTime: ts,
				UpdatedTime: ts,
			},
			Holders: holders,
		}
		if err = ab.PutAccount(account); err != nil {
			return nil, nil, errors.Wrap(err, "failed to create an account")
		}

		// balance
		bb := NewBalanceStub(ab.stub)
		balance, err := bb.CreateBalance(account.GetID())
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to create an account balance")
		}

		// create account-holder relationship
		for kid := range holders.Map() {
			holder := NewHolder(kid, account)
			holder.CreatedTime = ts
			if err = ab.PutHolder(holder); err != nil {
				return nil, nil, errors.Wrap(err, "failed to create the holder")
			}
		}

		return account, balance, nil
	}

	// hash collision (retry later)
	return nil, nil, errors.New("failed to create a random address")
}

// GetAccount retrieves the account by an address
func (ab *AccountStub) GetAccount(addr *Address) (AccountInterface, error) {
	data, err := ab.GetAccountState(addr.String())
	if err != nil {
		return nil, err
	}
	// data is not nil
	var account AccountInterface
	switch addr.Type {
	case AccountTypePersonal:
		account = &Account{}
	case AccountTypeJoint:
		account = &JointAccount{}
	default: // never here (addr has been validated)
		return nil, InvalidAccountAddrError{}
	}
	if err = json.Unmarshal(data, account); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the account")
	}
	return account, nil
}

// GetAccountState _
func (ab *AccountStub) GetAccountState(addr string) ([]byte, error) {
	data, err := ab.stub.GetState(ab.CreateKey(addr))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the account state")
	}
	if data != nil {
		return data, nil
	}
	return nil, NotExistedAccountError{addr: addr}
}

// GetQueryHolderAccounts _
func (ab *AccountStub) GetQueryHolderAccounts(kid, bookmark string, fetchSize int) (*QueryResult, error) {
	if fetchSize < 1 {
		fetchSize = AccountsFetchSize
	}
	if fetchSize > 200 {
		fetchSize = 200
	}
	query := ""
	if len(ab.token) > 0 {
		query = CreateQueryHoldersByIDAndTokenCode(kid, ab.token)
	} else {
		query = CreateQueryHoldersByID(kid)
	}
	iter, meta, err := ab.stub.GetQueryResultWithPagination(query, int32(fetchSize), bookmark)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	return NewQueryResult(meta, iter)
}

// GetSignableIDs returns holders' KID array
func (ab *AccountStub) GetSignableIDs(addr string) ([]string, error) {
	_addr, err := ParseAddress(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse the account address: [%s]", addr)
	}
	if _addr.Code != ab.token {
		return nil, errors.Errorf("mismatched token account: [%s]", addr)
	}
	account, err := ab.GetAccount(_addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the account: [%s]", addr)
	}
	if pac, ok := account.(*Account); ok {
		return []string{pac.Holder()}, nil
	}
	jac := account.(*JointAccount)
	return jac.Holders.Strings(), nil
}

// PutAccount _
func (ab *AccountStub) PutAccount(account AccountInterface) error {
	data, err := json.Marshal(account)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the account")
	}
	if err = ab.stub.PutState(ab.CreateKey(account.GetID()), data); err != nil {
		return errors.Wrap(err, "failed to put the account state")
	}
	return nil
}

// SuspendAccount _
func (ab *AccountStub) SuspendAccount(kid string) (*Account, error) {
	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	account, err := ab.GetAccount(NewAddress(ab.token, AccountTypePersonal, kid))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the personal account")
	}
	pac := account.(*Account)
	if pac.SuspendedTime != nil {
		return nil, errors.New("already suspended")
	}

	pac.SuspendedTime = ts
	pac.UpdatedTime = ts
	if err = ab.PutAccount(pac); err != nil {
		return nil, errors.Wrap(err, "failed to update the account")
	}
	return pac, nil
}

// UnsuspendAccount _
func (ab *AccountStub) UnsuspendAccount(kid string) (*Account, error) {
	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	account, err := ab.GetAccount(NewAddress(ab.token, AccountTypePersonal, kid))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the personal account")
	}
	pac := account.(*Account)
	if nil == pac.SuspendedTime {
		return nil, errors.New("not suspended")
	}

	pac.SuspendedTime = nil
	pac.UpdatedTime = ts
	if err = ab.PutAccount(pac); err != nil {
		return nil, errors.Wrap(err, "failed to update the account")
	}
	return pac, nil
}

// CreateHolderKey _
func (ab *AccountStub) CreateHolderKey(id, addr string) string {
	return fmt.Sprintf("HLD_%s_%s", id, addr)
}

// PutHolder _
func (ab *AccountStub) PutHolder(holder *Holder) error {
	data, err := json.Marshal(holder)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the holder")
	}
	if err = ab.stub.PutState(ab.CreateHolderKey(holder.DOCTYPEID, holder.Address), data); err != nil {
		return errors.Wrap(err, "failed to put the holder state")
	}
	return nil
}

// AddHolder _
func (ab *AccountStub) AddHolder(account *JointAccount, kid string) (*JointAccount, error) {
	if account.HasHolder(kid) {
		return nil, errors.New("already existed holder")
	}

	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	account.Holders.Add(kid)
	account.UpdatedTime = ts
	if err = ab.PutAccount(account); err != nil {
		return nil, errors.Wrap(err, "failed to update the account")
	}

	// cretae account-holder relationship
	holder := NewHolder(kid, account)
	holder.CreatedTime = ts
	if err = ab.PutHolder(holder); err != nil {
		return nil, errors.Wrap(err, "failed to create the relationship")
	}

	return account, nil
}

// RemoveHolder _
func (ab *AccountStub) RemoveHolder(account *JointAccount, kid string) (*JointAccount, error) {
	if !account.HasHolder(kid) {
		return nil, errors.New("not existed holder")
	}

	ts, err := txtime.GetTime(ab.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	account.Holders.Remove(kid)
	account.UpdatedTime = ts
	if err = ab.PutAccount(account); err != nil {
		return nil, errors.Wrap(err, "failed to update the account")
	}

	// remove account-holder relationship
	if err = ab.stub.DelState(ab.CreateHolderKey(kid, account.GetID())); err != nil {
		return nil, errors.Wrap(err, "failed to delete the relationship")
	}

	return account, nil
}
