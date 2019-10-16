package main

import (
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// LastPrunedFeeIDStub _
type LastPrunedFeeIDStub struct {
	stub shim.ChaincodeStubInterface
}

// NewLastPrunedFeeIDStub _
func NewLastPrunedFeeIDStub(stub shim.ChaincodeStubInterface) *LastPrunedFeeIDStub {
	return &LastPrunedFeeIDStub{stub}
}

// CreateKey _
func (lb *LastPrunedFeeIDStub) CreateKey(tokenCode string) string {
	return "LPF_" + tokenCode
}

// CreateLastPrunedFeeID created a new LastPrunedFeeID document and puts it in the world state.
// feeID of legacy Token.LastPrunedFeeID is required at the first time.
func (lb *LastPrunedFeeIDStub) CreateLastPrunedFeeID(tokenCode, feeID string) (*LastPrunedFeeID, error) {
	ts, err := txtime.GetTime(lb.stub)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the timestamp")
	}

	l := &LastPrunedFeeID{
		DOCTYPEID:   tokenCode,
		FeeID:       feeID,
		UpdatedTime: ts,
	}
	if err = lb.PutLastPrunedFeeID(l); err != nil {
		return nil, err
	}
	return l, nil
}

// GetLastPrunedFeeID _
func (lb *LastPrunedFeeIDStub) GetLastPrunedFeeID(code string) (*LastPrunedFeeID, error) {
	data, err := lb.GetLastPrunedFeeIDState(code)
	if err != nil {
		return nil, err
	}
	// data is not nil
	token := &LastPrunedFeeID{}
	if err = json.Unmarshal(data, token); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal the last pruned fee id")
	}
	return token, nil
}

// GetLastPrunedFeeIDState _
func (lb *LastPrunedFeeIDStub) GetLastPrunedFeeIDState(tokenCode string) ([]byte, error) {
	data, err := lb.stub.GetState(lb.CreateKey(tokenCode))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the last pruned fee id state")
	}
	if data != nil {
		return data, nil
	}
	return nil, NotInitLastPrunedFeeIDError{tokenCode: tokenCode}
}

// PutLastPrunedFeeID _
func (lb *LastPrunedFeeIDStub) PutLastPrunedFeeID(l *LastPrunedFeeID) error {
	data, err := json.Marshal(l)
	if err != nil {
		return errors.Wrap(err, "failed to marshal the last pruned fee id")
	}
	if err = lb.stub.PutState(lb.CreateKey(l.DOCTYPEID), data); err != nil {
		return errors.Wrap(err, "failed to put the last pruned fee id state")
	}
	return nil
}
