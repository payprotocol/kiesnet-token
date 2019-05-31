// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"strings"

	"github.com/key-inside/kiesnet-ccpkg/stringset"
	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// AccountInterface _
type AccountInterface interface {
	Identifiable
	GetToken() string
	GetType() AccountType
	HasHolder(kid string) bool
	IsSuspended() bool
}

// AccountType _
type AccountType byte

const (
	// AccountTypeUnknown _
	AccountTypeUnknown AccountType = iota
	// AccountTypePersonal _
	AccountTypePersonal
	// AccountTypeJoint _
	AccountTypeJoint
)

// Account _
type Account struct {
	DOCTYPEID     string       `json:"@account"` // address
	Token         string       `json:"token"`
	Type          AccountType  `json:"type"`
	CreatedTime   *txtime.Time `json:"created_time,omitempty"`
	UpdatedTime   *txtime.Time `json:"updated_time,omitempty"`
	SuspendedTime *txtime.Time `json:"suspended_time,omitempty"`
}

// GetID implements Identifiable
func (a *Account) GetID() string {
	return a.DOCTYPEID
}

// GetToken implements AccountInterface
func (a *Account) GetToken() string {
	return a.Token
}

// GetType implements AccountInterface
func (a *Account) GetType() AccountType {
	return a.Type
}

// HasHolder implements AccountInterface
func (a *Account) HasHolder(kid string) bool {
	return a.Holder() == kid
}

// IsSuspended implements AccountInterface
func (a *Account) IsSuspended() bool {
	return a.SuspendedTime != nil
}

// Holder returns holder's KID
func (a *Account) Holder() string {
	i := len(a.DOCTYPEID) - 48
	return strings.ToLower(a.DOCTYPEID[i : i+40])
}

// JointAccount _
type JointAccount struct {
	Account
	Holders *stringset.Set `json:"holders"`
}

// HasHolder implements AccountInterface
func (a *JointAccount) HasHolder(kid string) bool {
	return a.Holders.Contains(kid)
}

// Holder override
func (a *JointAccount) Holder() string {
	return ""
}

// Holder represents an account-holder relationship (many-to-many)
type Holder struct {
	DOCTYPEID   string       `json:"@holder"`
	Address     string       `json:"address"`
	Token       string       `json:"token"`
	Type        AccountType  `json:"type"`
	CreatedTime *txtime.Time `json:"created_time,omitempty"`
}

// NewHolder _
func NewHolder(kid string, account AccountInterface) *Holder {
	return &Holder{
		DOCTYPEID: kid,
		Address:   account.GetID(),
		Token:     account.GetToken(),
		Type:      account.GetType(),
	}
}
