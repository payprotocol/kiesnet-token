// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// PaysPruneSize _
// XXX *** StateDB fetch limit MUST be greater than PaysPruneSize
const PaysPruneSize = 900

// PaysFetchSize _
const PaysFetchSize = 20

// PayStub _
type PayStub struct {
	stub shim.ChaincodeStubInterface
}

// NewPayStub _
func NewPayStub(stub shim.ChaincodeStubInterface) *PayStub {
	return &PayStub{stub}
}

// CreateKey _
func (pb *PayStub) CreateKey(id string) string {
	if id == "" {
		return ""
	}
	return fmt.Sprintf("PAY_%s", id)
}

// GetPay _
func (pb *PayStub) GetPay(id string) (*Pay, error) {
	data, err := pb.GetPayState(id)
	if nil != err {
		return nil, err
	}
	// data is not nil
	pay := &Pay{}
	if err = json.Unmarshal(data, pay); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the pay")
	}
	return pay, nil
}

// GetPayState _
func (pb *PayStub) GetPayState(id string) ([]byte, error) {
	data, err := pb.stub.GetState(pb.CreateKey(id))
	if nil != err {
		return nil, errors.Wrap(err, "failed to get the pay state")
	}
	if nil != data {
		return data, nil
	}
	return nil, NotExistedPayError{id: id}
}

// GetPayByOrderID retrieves Pay by vendor specific order id field.
func (pb *PayStub) GetPayByOrderID(orderID string) (*Pay, error) {
	query := CreateQueryPayByOrderID(orderID)
	iter, err := pb.stub.GetQueryResult(query)
	if nil != err {
		return nil, err
	}
	defer iter.Close()

	if !iter.HasNext() {
		return nil, errors.New("cannot find pay of the order id")
	}
	kv, err := iter.Next()
	if nil != err {
		return nil, err
	}
	pay := &Pay{}
	err = json.Unmarshal(kv.Value, pay)
	if nil != err {
		return nil, err
	}
	return pay, nil
}

// PutPay _
func (pb *PayStub) PutPay(pay *Pay) error {
	data, err := json.Marshal(pay)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the balance")
	}
	if err = pb.stub.PutState(pb.CreateKey(pay.PayID), data); err != nil {
		return errors.Wrap(err, "failed to put the balance state")
	}
	return nil
}

// PutParentPay _
func (pb *PayStub) PutParentPay(key string, pay *Pay) error {
	data, err := json.Marshal(pay)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the balance")
	}
	if err = pb.stub.PutState(key, data); err != nil {
		return errors.Wrap(err, "failed to put the balance state")
	}
	return nil
}

// Pay _
func (pb *PayStub) Pay(sender *Balance, receiver string, amount, fee Amount, orderID, memo string) (*PayResult, error) {
	ts, err := txtime.GetTime(pb.stub)
	if nil != err {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}
	payid := fmt.Sprintf("%d%s", ts.UnixNano(), pb.stub.GetTxID())
	pay := NewPay(receiver, payid, amount, fee, sender.GetID(), "", orderID, memo, ts)
	if err = pb.PutPay(pay); nil != err {
		return nil, errors.Wrap(err, "failed to put new pay")
	}

	amount.Neg()
	sender.Amount.Add(&amount)
	sender.UpdatedTime = ts
	if err = NewBalanceStub(pb.stub).PutBalance(sender); nil != err {
		return nil, errors.Wrap(err, "failed to update sender balance")
	}

	var sbl *BalanceLog
	sbl = NewBalancePayLog(sender, pay)
	sbl.CreatedTime = ts
	if err = NewBalanceStub(pb.stub).PutBalanceLog(sbl); err != nil {
		return nil, errors.Wrap(err, "failed to update sender balance log")
	}

	return NewPayResult(pay, sbl), nil
}

// Refund _
func (pb *PayStub) Refund(sender, receiver *Balance, amount, fee Amount, memo string, parentPay *Pay) (*BalanceLog, error) {
	ts, err := txtime.GetTime(pb.stub)
	if nil != err {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	payid := fmt.Sprintf("%d%s", ts.UnixNano(), pb.stub.GetTxID()) //TODO: fee neg
	pay := NewPay(sender.GetID(), payid, *amount.Copy().Neg(), *fee.Copy().Neg(), receiver.GetID(), parentPay.PayID, "", memo, ts)
	if err = pb.PutPay(pay); nil != err {
		return nil, errors.Wrap(err, "failed to put new refund")
	}

	//update the total refund amount to the parent pay
	parentPay.TotalRefund = *parentPay.TotalRefund.Add(&amount)
	if err = pb.PutParentPay(pb.CreateKey(parentPay.PayID), parentPay); err != nil {
		return nil, errors.Wrap(err, "failed to update parent pay")
	}

	// refund
	bb := NewBalanceStub(pb.stub)

	receiver.Amount.Add(&amount)
	receiver.UpdatedTime = ts
	if err = bb.PutBalance(receiver); nil != err {
		return nil, errors.Wrap(err, "failed to update receiver balance")
	}

	rbl := NewBalanceRefundLog(receiver, pay)
	rbl.CreatedTime = ts
	if err = bb.PutBalanceLog(rbl); err != nil {
		return nil, errors.Wrap(err, "failed to update receiver's balance log")
	}

	return rbl, nil
}

// GetPaySumByTime _{end sum next}
func (pb *PayStub) GetPaySumByTime(id string, stime, etime *txtime.Time) (*PaySum, error) {
	query := CreateQueryPrunePays(id, stime, etime)
	iter, err := pb.stub.GetQueryResult(query)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	cs := &PaySum{HasMore: false}
	c := &Pay{}
	cnt := 0 //record counter
	sum, _ := NewAmount("0")
	fee, _ := NewAmount("0")

	for iter.HasNext() {
		cnt++
		kv, err := iter.Next()
		if nil != err {
			return nil, err
		}

		err = json.Unmarshal(kv.Value, c)
		if err != nil {
			return nil, err
		}

		if 1 == cnt {
			cs.Start = c.PayID
		}

		if cnt > PaysPruneSize {
			cs.HasMore = true
			cnt--
			break
		}
		sum = sum.Add(&c.Amount)
		fee = fee.Add(&c.Fee)
		cs.End = c.PayID
	}
	cs.Count = cnt
	cs.Sum = sum
	cs.Fee = fee

	return cs, nil
}

// GetPaysByTime _
func (pb *PayStub) GetPaysByTime(id, sortOrder, bookmark string, stime, etime *txtime.Time, fetchSize int) (*QueryResult, error) {
	if fetchSize < 1 {
		fetchSize = PaysFetchSize
	}
	if fetchSize > 200 {
		fetchSize = 200
	}
	query := ""
	if stime != nil || etime != nil {
		query = CreateQueryPaysByIDAndTime(id, sortOrder, stime, etime)
	} else {
		query = CreateQueryPaysByID(id, sortOrder)
	}
	iter, meta, err := pb.stub.GetQueryResultWithPagination(query, int32(fetchSize), bookmark)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	return NewQueryResult(meta, iter)
}

// PayPendingBalance _
func (pb *PayStub) PayPendingBalance(pbalance *PendingBalance, fee Amount, merchant, memo, orderID string) error {
	ts, err := txtime.GetTime(pb.stub)
	if nil != err {
		return err
	}

	payid := fmt.Sprintf("%d%s", ts.UnixNano(), pb.stub.GetTxID())

	// Put pay
	pay := NewPay(merchant, payid, pbalance.Amount, fee, pbalance.Account, "", orderID, memo, ts)
	if err = pb.PutPay(pay); nil != err {
		return errors.Wrap(err, "failed to put new pay")
	}

	// remove pending balance
	if err := pb.stub.DelState(NewBalanceStub(pb.stub).CreatePendingKey(pbalance.DOCTYPEID)); err != nil {
		return errors.Wrap(err, "failed to delete the pending balance")
	}
	return nil
}
