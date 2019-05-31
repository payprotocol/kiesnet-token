// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/key-inside/kiesnet-ccpkg/txtime"
	"github.com/pkg/errors"
)

// Fee is a transfer/pay fee utxo which will be pruned to genesis account
type Fee struct {
	DOCTYPEID   string       `json:"@fee,required"` // token code
	FeeID       string       `json:"fee_id"`        // unique sequential identifier (timestamp + txid)
	Account     string       `json:"account"`       // account address who payed fee
	Amount      Amount       `json:"amount"`
	CreatedTime *txtime.Time `json:"created_time"`
}

// FeePolicy _
type FeePolicy struct {
	TargetAddress string             `json:"target_address"`
	Rates         map[string]FeeRate `json:"rates"`
}

// isValidFn returns true if given fn is defined.
func isValidFn(fn string) bool {
	switch fn {
	case "transfer":
		fallthrough
	case "pay": // All valid fee rate type case should fallthrough here, the last one.
		return true
	}
	return false
}

// ParseFeePolicy parses fee policy format string to FeePolicy struct.
func ParseFeePolicy(s string) (policy *FeePolicy, err error) {
	// fees -> map
	rates := map[string]FeeRate{}
	fees := strings.Split(s, ";")
	for _, f := range fees {
		kv := strings.Split(f, "=")
		if len(kv) > 1 {
			// We limit fn(= kv[0]) to one of "transfer" or "pay".
			if valid := isValidFn(kv[0]); !valid {
				return nil, errors.New("invalid fee rate type")
			}
			rm := strings.Split(kv[1], ",")
			rate := rm[0]
			if _, ok := new(big.Rat).SetString(rate); !ok {
				return nil, errors.New("failed to parse rate")
			}
			max := int64(0)
			if len(rm) > 1 {
				max, err = strconv.ParseInt(rm[1], 10, 64)
				if err != nil {
					return nil, errors.New("failed to parse max fee amount")
				}
			}
			rates[kv[0]] = FeeRate{
				Rate:      rate,
				MaxAmount: max,
			}
		}
	}
	policy = &FeePolicy{
		Rates: rates,
	}
	return
}

// FeeRate _
type FeeRate struct {
	Rate      string `json:"rate"`       // numeric string of positive decimal fraction
	MaxAmount int64  `json:"max_amount"` // 0 is unlimit
}

// FeeSum stands for amount&state of accumulated fee from Start to End
type FeeSum struct {
	Sum     *Amount `json:"sum"`
	Count   int     `json:"count"`
	Start   string  `json:"start_id"`
	End     string  `json:"end_id"`
	HasMore bool    `json:"has_more"`
}
