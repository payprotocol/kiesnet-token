// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"github.com/key-inside/kiesnet-ccpkg/contract"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// Balance _
type Balance struct {
	DOCTYPEID       string       `json:"@balance"` // address
	Amount          Amount       `json:"amount"`
	CreatedTime     *txtime.Time `json:"created_time,omitempty"`
	UpdatedTime     *txtime.Time `json:"updated_time,omitempty"`
	LastPrunedPayID string       `json:"last_pruned_pay_id,omitempty"`
}

// GetID implements Identifiable
func (b *Balance) GetID() string {
	return b.DOCTYPEID
}

// BalanceLogType _
type BalanceLogType int8

const (
	// BalanceLogTypeMint _
	BalanceLogTypeMint BalanceLogType = iota
	// BalanceLogTypeBurn _
	BalanceLogTypeBurn
	// BalanceLogTypeSend _
	BalanceLogTypeSend
	// BalanceLogTypeReceive _
	BalanceLogTypeReceive
	// BalanceLogTypeDeposit deposit balance to contract
	BalanceLogTypeDeposit
	// BalanceLogTypeWithdraw withdraw balance from contract
	BalanceLogTypeWithdraw
	// BalanceLogTypePay pay amount of balance
	BalanceLogTypePay
	// BalanceLogTypeRefund refund amount of balance
	BalanceLogTypeRefund
	// BalanceLogTypePrunePay the amount of pruned payments
	BalanceLogTypePrunePay
	// BalanceLogTypePruneFee is created when fee utxos are pruned to genesis account.
	BalanceLogTypePruneFee
)

// BalanceLog _
type BalanceLog struct {
	DOCTYPEID    string         `json:"@balance_log"` // address
	Type         BalanceLogType `json:"type"`
	RID          string         `json:"rid"` // relative ID
	Diff         Amount         `json:"diff"`
	Fee          *Amount        `json:"fee,omitempty"`
	Amount       Amount         `json:"amount"`
	Memo         string         `json:"memo"`
	CreatedTime  *txtime.Time   `json:"created_time,omitempty"`
	PruneStartID string         `json:"prune_start_id,omitempty"` // used for pruned balance log
	PruneEndID   string         `json:"prune_end_id,omitempty"`   // used for pruned balance log
	PayID        string         `json:"pay_id,omitempty"`         // used for pay balance log
}

// MemoMaxLength is used to limit memo field length (BalanceLog, PendingBalance, Pay)
const MemoMaxLength = 1024

// NewBalanceSupplyLog _
func NewBalanceSupplyLog(bal *Balance, diff Amount) *BalanceLog {
	if diff.Sign() < 0 { // burn
		return &BalanceLog{
			DOCTYPEID: bal.DOCTYPEID,
			Type:      BalanceLogTypeBurn,
			Diff:      diff,
			Amount:    bal.Amount,
		}
	} // else  mint
	return &BalanceLog{
		DOCTYPEID: bal.DOCTYPEID,
		Type:      BalanceLogTypeMint,
		Diff:      diff,
		Amount:    bal.Amount,
	}
}

// NewBalanceTransferLog _
func NewBalanceTransferLog(sender, receiver *Balance, diff Amount, fee *Amount, memo string) *BalanceLog {
	if diff.Sign() < 0 { // sender log
		return &BalanceLog{
			DOCTYPEID: sender.DOCTYPEID,
			Type:      BalanceLogTypeSend,
			RID:       receiver.DOCTYPEID,
			Diff:      diff,
			Fee:       fee,
			Amount:    sender.Amount,
			Memo:      memo,
		}
	} // else receiver log
	return &BalanceLog{
		DOCTYPEID: receiver.DOCTYPEID,
		Type:      BalanceLogTypeReceive,
		RID:       sender.DOCTYPEID,
		Diff:      diff,
		Amount:    receiver.Amount,
		Memo:      memo,
	}
}

// NewBalanceDepositLog _
func NewBalanceDepositLog(bal *Balance, pb *PendingBalance) *BalanceLog {
	diff := pb.Amount.Copy().Neg()
	return &BalanceLog{
		DOCTYPEID: bal.DOCTYPEID,
		Type:      BalanceLogTypeDeposit,
		RID:       pb.RID,
		Diff:      *diff,
		Fee:       pb.Fee,
		Amount:    bal.Amount,
		Memo:      pb.Memo,
	}
}

// NewBalanceWithdrawLog _
func NewBalanceWithdrawLog(bal *Balance, pb *PendingBalance) *BalanceLog {
	diff := pb.Amount.Copy()
	if pb.Fee != nil {
		diff = diff.Add(pb.Fee)
	}
	return &BalanceLog{
		DOCTYPEID: bal.DOCTYPEID,
		Type:      BalanceLogTypeWithdraw,
		RID:       pb.RID,
		Diff:      *diff,
		Amount:    bal.Amount,
		Memo:      pb.Memo,
	}
}

// NewBalancePayLog _
func NewBalancePayLog(bal *Balance, pay *Pay) *BalanceLog {
	diff := pay.Amount.Copy().Neg()
	return &BalanceLog{
		DOCTYPEID: bal.DOCTYPEID,
		Type:      BalanceLogTypePay,
		RID:       pay.DOCTYPEID,
		Diff:      *diff,
		Amount:    bal.Amount,
		Memo:      pay.Memo,
		PayID:     pay.PayID,
	}
}

// NewBalanceRefundLog _
func NewBalanceRefundLog(bal *Balance, pay *Pay) *BalanceLog {
	diff := pay.Amount.Copy().Neg()
	return &BalanceLog{
		DOCTYPEID: bal.DOCTYPEID,
		Type:      BalanceLogTypeRefund,
		RID:       pay.DOCTYPEID,
		Diff:      *diff,
		Amount:    bal.Amount,
		Memo:      pay.Memo,
		// PayID ?
	}
}

// NewBalancePrunePayLog No need RID
func NewBalancePrunePayLog(bal *Balance, amount Amount, Start, End string) *BalanceLog {
	return &BalanceLog{
		DOCTYPEID:    bal.DOCTYPEID,
		Type:         BalanceLogTypePrunePay,
		Diff:         amount,
		Amount:       bal.Amount,
		PruneStartID: Start,
		PruneEndID:   End,
	}
}

// NewBalancePruneFeeLog creates new BalanceLog of type BalanceLogTypePruneFee
func NewBalancePruneFeeLog(bal *Balance, amount Amount, Start, End string) *BalanceLog {
	return &BalanceLog{
		DOCTYPEID:    bal.DOCTYPEID,
		Type:         BalanceLogTypePruneFee,
		Diff:         amount,
		Amount:       bal.Amount,
		PruneStartID: Start,
		PruneEndID:   End,
	}
}

// PendingBalanceType _
type PendingBalanceType int8

const (
	// PendingBalanceTypeAccount _
	PendingBalanceTypeAccount PendingBalanceType = iota
	// PendingBalanceTypeContract _
	PendingBalanceTypeContract
)

// PendingBalance _
type PendingBalance struct {
	DOCTYPEID   string             `json:"@pending_balance"` // id
	Type        PendingBalanceType `json:"type"`
	Account     string             `json:"account"` // account ID (address)
	RID         string             `json:"rid"`     // relative ID - account or contract
	Amount      Amount             `json:"amount"`
	Fee         *Amount            `json:"fee,omitempty"`
	Memo        string             `json:"memo"`
	CreatedTime *txtime.Time       `json:"created_time,omitempty"`
	PendingTime *txtime.Time       `json:"pending_time,omitempty"`
}

// NewPendingBalance _
func NewPendingBalance(id string, owner Identifiable, rel Identifiable, amount Amount, fee *Amount, memo string, pTime *txtime.Time) *PendingBalance {
	ptype := PendingBalanceTypeAccount
	if _, ok := rel.(*contract.Contract); ok {
		ptype = PendingBalanceTypeContract
	}
	return &PendingBalance{
		DOCTYPEID:   id,
		Type:        ptype,
		Account:     owner.GetID(),
		RID:         rel.GetID(),
		Amount:      amount,
		Fee:         fee,
		Memo:        memo,
		PendingTime: pTime,
	}
}
