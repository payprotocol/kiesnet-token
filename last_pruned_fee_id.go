package main

import "github.com/key-inside/kiesnet-ccpkg/txtime"

// LastPrunedFeeID is a last pruned fee id of the token.
type LastPrunedFeeID struct {
	DOCTYPEID   string       `json:"@last_pruned_fee_id"`
	FeeID       string       `json:"fee_id"`
	UpdatedTime *txtime.Time `json:"updated_time,omitempty"`
}
