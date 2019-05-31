// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// Pay _
type Pay struct {
	DOCTYPEID   string       `json:"@pay"`                   //address
	PayID       string       `json:"pay_id"`                 //used for the external client to pass the pay id for refund
	Amount      Amount       `json:"amount"`                 //can be positive(pay) or negative(refund)
	Fee         Amount       `json:"fee"`                    //can be positive(pay) or negative(refund, to return fee to merchant when she prune her pays)
	TotalRefund Amount       `json:"total_refund,omitempty"` //total refund value
	RID         string       `json:"rid"`                    //related id. user who pays to the merchant or receives refund from the merchant.
	ParentID    string       `json:"parent_id,omitempty"`    //parent id. this value exists only when the pay type is refund(negative amount)
	OrderID     string       `json:"order_id,omitempty"`     // order id. vendor specific unique identifier.
	Memo        string       `json:"memo"`
	CreatedTime *txtime.Time `json:"created_time,omitempty"`
}

// NewPay _
func NewPay(id, payid string, amount, fee Amount, rid, pid, orderID, memo string, ts *txtime.Time) *Pay {
	return &Pay{
		DOCTYPEID:   id,
		PayID:       payid,
		Amount:      amount,
		Fee:         fee,
		RID:         rid,
		ParentID:    pid,
		OrderID:     orderID,
		Memo:        memo,
		CreatedTime: ts,
	}
}

// PaySum _
type PaySum struct {
	Sum     *Amount `json:"sum"`
	Fee     *Amount `json:"fee"` // sum of Pay.Fee between Start and End
	Count   int     `json:"prune_count"`
	Start   string  `json:"start_id"`
	End     string  `json:"end_id"`
	HasMore bool    `json:"has_more"`
}

// PayResult _
type PayResult struct {
	Pay        *Pay        `json:"pay"`
	BalanceLog *BalanceLog `json:"balance_log"`
}

// NewPayResult _
func NewPayResult(pay *Pay, log *BalanceLog) *PayResult {
	return &PayResult{
		Pay:        pay,
		BalanceLog: log,
	}
}
