// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"bytes"
	"math/big"

	"github.com/pkg/errors"
)

// Amount _
type Amount struct {
	big.Int
}

// NewAmount _
func NewAmount(val string) (*Amount, error) {
	a := &Amount{}
	if len(val) > 0 {
		if _, ok := a.SetString(val, 10); !ok {
			return nil, errors.New("invalid amount value: must be integer")
		}
	}
	return a, nil
}

// NewAmountWithBigInt _
func NewAmountWithBigInt(bigInt *big.Int) *Amount {
	return &Amount{Int: *bigInt}
}

// ZeroAmount _
func ZeroAmount() *Amount {
	return &Amount{}
}

// Add override
func (a *Amount) Add(x *Amount) *Amount {
	a.Int.Add(&a.Int, &x.Int)
	return a
}

// Cmp override
func (a *Amount) Cmp(x *Amount) int {
	return a.Int.Cmp(&x.Int)
}

// Copy _
func (a *Amount) Copy() *Amount {
	n := &Amount{}
	return n.Add(a)
}

// Neg override
func (a *Amount) Neg() *Amount {
	a.Int.Neg(&a.Int)
	return a
}

// MulRat _
func (a *Amount) MulRat(r *big.Rat) *Amount {
	// floor
	a.Int.Div(a.Int.Mul(&a.Int, r.Num()), r.Denom())
	return a
}

// MarshalJSON override
func (a *Amount) MarshalJSON() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{'"'})
	if _, err := buf.WriteString(a.String()); err != nil {
		return nil, err
	}
	if err := buf.WriteByte('"'); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalJSON override
func (a *Amount) UnmarshalJSON(text []byte) error {
	return a.Int.UnmarshalJSON(text[1 : len(text)-1])
}
