// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/key-inside/kiesnet-ccpkg/contract"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// BalanceLogsFetchSize _
const BalanceLogsFetchSize = 20

// PendingBalancesFetchSize _
const PendingBalancesFetchSize = 20

// BalanceStub _
type BalanceStub struct {
	stub shim.ChaincodeStubInterface
}

// NewBalanceStub _
func NewBalanceStub(stub shim.ChaincodeStubInterface) *BalanceStub {
	return &BalanceStub{stub}
}

// CreateKey _
func (bb *BalanceStub) CreateKey(id string) string {
	return "BLC_" + id
}

// CreateBalance _
func (bb *BalanceStub) CreateBalance(id string) (*Balance, error) {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	balance := &Balance{}
	balance.DOCTYPEID = id
	balance.CreatedTime = ts
	balance.UpdatedTime = ts
	if err = bb.PutBalance(balance); err != nil {
		return nil, errors.Wrap(err, "failed to create a balance")
	}

	return balance, nil
}

// GetBalance _
func (bb *BalanceStub) GetBalance(id string) (*Balance, error) {
	data, err := bb.GetBalanceState(id)
	if err != nil {
		return nil, err
	}
	// data is not nil
	balance := &Balance{}
	if err = json.Unmarshal(data, balance); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the balance")
	}
	return balance, nil
}

// GetBalanceState _
func (bb *BalanceStub) GetBalanceState(id string) ([]byte, error) {
	data, err := bb.stub.GetState(bb.CreateKey(id))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the balance state")
	}
	if data != nil {
		return data, nil
	}
	// ISSUE: failover
	logger.Criticalf("nil balance: %s", id)
	return nil, errors.New("balance is not exists")
}

// GetQueryBalanceLogs _
func (bb *BalanceStub) GetQueryBalanceLogs(id, typeStr, bookmark string, fetchSize int, stime, etime *txtime.Time) (*QueryResult, error) {
	if fetchSize < 1 {
		fetchSize = BalanceLogsFetchSize
	}
	if fetchSize > 200 {
		fetchSize = 200
	}
	query := ""
	if stime != nil || etime != nil {
		query = CreateQueryBalanceLogsByIDAndTimes(id, typeStr, stime, etime)
	} else {
		query = CreateQueryBalanceLogsByID(id, typeStr)
	}
	iter, meta, err := bb.stub.GetQueryResultWithPagination(query, int32(fetchSize), bookmark)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	return NewQueryResult(meta, iter)
}

// PutBalance _
func (bb *BalanceStub) PutBalance(balance *Balance) error {
	data, err := json.Marshal(balance)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the balance")
	}
	if err = bb.stub.PutState(bb.CreateKey(balance.DOCTYPEID), data); err != nil {
		return errors.Wrap(err, "failed to put the balance state")
	}
	return nil
}

// CreateLogKey _
func (bb *BalanceStub) CreateLogKey(id string, seq int64) string {
	return fmt.Sprintf("BLOG_%s_%d", id, seq)
}

// PutBalanceLog _
func (bb *BalanceStub) PutBalanceLog(log *BalanceLog) error {
	data, err := json.Marshal(log)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the balance log")
	}
	if err = bb.stub.PutState(bb.CreateLogKey(log.DOCTYPEID, log.CreatedTime.UnixNano()), data); err != nil {
		return errors.Wrap(err, "failed to put the balance log state")
	}
	return nil
}

// CreatePendingKey _
func (bb *BalanceStub) CreatePendingKey(id string) string {
	return "PBLC_" + id
}

// GetPendingBalance _
func (bb *BalanceStub) GetPendingBalance(id string) (*PendingBalance, error) {
	data, err := bb.stub.GetState(bb.CreatePendingKey(id))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the pending balance state")
	}
	if data != nil {
		balance := &PendingBalance{}
		if err = json.Unmarshal(data, balance); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal the balance")
		}
		return balance, nil
	}
	return nil, errors.New("the pending balance is not exists")
}

// GetQueryPendingBalances _
func (bb *BalanceStub) GetQueryPendingBalances(addr, sort, bookmark string, fetchSize int) (*QueryResult, error) {
	if fetchSize < 1 {
		fetchSize = PendingBalancesFetchSize
	}
	if fetchSize > 200 {
		fetchSize = 200
	}
	query := CreateQueryPendingBalancesByAddress(addr, sort)
	iter, meta, err := bb.stub.GetQueryResultWithPagination(query, int32(fetchSize), bookmark)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	return NewQueryResult(meta, iter)
}

// PutPendingBalance _
func (bb *BalanceStub) PutPendingBalance(balance *PendingBalance) error {
	data, err := json.Marshal(balance)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the pending balance")
	}
	if err = bb.stub.PutState(bb.CreatePendingKey(balance.DOCTYPEID), data); err != nil {
		return errors.Wrap(err, "failed to put the pending balance state")
	}
	return nil
}

// Supply - Mint & Burn
func (bb *BalanceStub) Supply(bal *Balance, amount Amount) (*BalanceLog, error) {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	bal.Amount.Add(&amount)
	bal.UpdatedTime = ts
	if err = bb.PutBalance(bal); err != nil {
		return nil, err
	}

	log := NewBalanceSupplyLog(bal, amount)
	log.CreatedTime = ts
	if err = bb.PutBalanceLog(log); err != nil {
		return nil, err
	}
	return log, nil
}

// Transfer _
func (bb *BalanceStub) Transfer(sender, receiver *Balance, amount, fee Amount, memo string, pendingTime *txtime.Time) (*BalanceLog, error) {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	if pendingTime != nil && pendingTime.Cmp(ts) > 0 { // time lock
		pb := NewPendingBalance(bb.stub.GetTxID(), receiver, sender, amount, nil, memo, pendingTime)
		pb.CreatedTime = ts
		if err = bb.PutPendingBalance(pb); err != nil {
			return nil, err
		}
	} else {
		receiver.Amount.Add(&amount) // deposit
		receiver.UpdatedTime = ts
		if err = bb.PutBalance(receiver); err != nil {
			return nil, err
		}
		rbl := NewBalanceTransferLog(sender, receiver, amount, nil, memo)
		rbl.CreatedTime = ts
		if err = bb.PutBalanceLog(rbl); err != nil {
			return nil, err
		}
	}

	amount.Neg()                        // -
	sender.Amount.Add(&amount)          // withdraw
	sender.Amount.Add(fee.Copy().Neg()) // fee
	sender.UpdatedTime = ts
	if err = bb.PutBalance(sender); err != nil {
		return nil, err
	}
	sbl := NewBalanceTransferLog(sender, receiver, amount, &fee, memo)
	sbl.CreatedTime = ts
	if err = bb.PutBalanceLog(sbl); err != nil {
		return nil, err
	}

	// fee
	if _, err := NewFeeStub(bb.stub).CreateFee(sender.GetID(), fee); err != nil {
		return nil, err
	}

	return sbl, nil
}

// TransferPendingBalance transfers the sender's pending balance. (multi-sig contract)
func (bb *BalanceStub) TransferPendingBalance(pb *PendingBalance, receiver *Balance, pendingTime *txtime.Time) error {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return errors.Wrap(err, "failed to get the timestamp")
	}

	sender := &Balance{DOCTYPEID: pb.Account} // proxy

	if pendingTime != nil && pendingTime.Cmp(ts) > 0 { // time lock
		pb := NewPendingBalance(bb.stub.GetTxID(), receiver, sender, pb.Amount, nil, pb.Memo, pendingTime)
		pb.CreatedTime = ts
		if err = bb.PutPendingBalance(pb); err != nil {
			return err
		}
	} else {
		receiver.Amount.Add(&pb.Amount) // deposit
		receiver.UpdatedTime = ts
		if err = bb.PutBalance(receiver); err != nil {
			return err
		}
		rbl := NewBalanceTransferLog(sender, receiver, pb.Amount, nil, pb.Memo)
		rbl.CreatedTime = ts
		if err = bb.PutBalanceLog(rbl); err != nil {
			return err
		}
	}

	// fee
	if pb.Fee != nil {
		if _, err := NewFeeStub(bb.stub).CreateFee(pb.Account, *pb.Fee); err != nil {
			return err
		}
	}

	// remove pending balance
	if err = bb.stub.DelState(bb.CreatePendingKey(pb.DOCTYPEID)); err != nil {
		return errors.Wrap(err, "failed to delete the pending balance")
	}

	return nil
}

// Deposit _
// It does not validate pending time!
func (bb *BalanceStub) Deposit(id string, sender *Balance, con *contract.Contract, amount Amount, fee *Amount, memo string) (*BalanceLog, error) {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	expiryTime, err := con.GetExpiryTime()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the expiry time")
	}

	pb := NewPendingBalance(id, sender, con, amount, fee, memo, expiryTime)
	pb.CreatedTime = ts
	if err = bb.PutPendingBalance(pb); err != nil {
		return nil, errors.Wrap(err, "failed to create the pending balance")
	}

	// applied = (amount + fee)
	applied := amount.Copy()
	if fee != nil {
		applied.Add(fee)
	}
	sender.Amount.Add(applied.Neg())	// -applied
	sender.UpdatedTime = ts
	if err = bb.PutBalance(sender); err != nil {
		return nil, err
	}
	log := NewBalanceDepositLog(sender, pb)
	log.CreatedTime = ts
	if err = bb.PutBalanceLog(log); err != nil {
		return nil, err
	}

	return log, nil
}

// Withdraw _
// It does not validate pending time!
func (bb *BalanceStub) Withdraw(pb *PendingBalance) (*BalanceLog, error) {
	ts, err := txtime.GetTime(bb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	bal, err := bb.GetBalance(pb.Account)
	if err != nil {
		return nil, err
	}
	applied := pb.Amount.Copy()
	if pb.Fee != nil {
		applied = applied.Add(pb.Fee)
	}
	bal.Amount.Add(applied)
	bal.UpdatedTime = ts
	if err = bb.PutBalance(bal); err != nil {
		return nil, err
	}
	log := NewBalanceWithdrawLog(bal, pb)
	log.CreatedTime = ts
	if err = bb.PutBalanceLog(log); err != nil {
		return nil, err
	}

	// remove pending balance
	if err = bb.stub.DelState(bb.CreatePendingKey(pb.DOCTYPEID)); err != nil {
		return nil, errors.Wrap(err, "failed to delete the pending balance")
	}

	return log, nil
}
