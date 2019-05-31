// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"bytes"
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"github.com/hyperledger/fabric/protos/peer"
)

// for fast JSON marsharing

// QueryResult _
type QueryResult struct {
	Meta    *peer.QueryResponseMetadata
	Records []byte
}

// NewQueryResult _
func NewQueryResult(meta *peer.QueryResponseMetadata, iter shim.StateQueryIteratorInterface) (*QueryResult, error) {
	result := &QueryResult{}
	result.Meta = meta

	buf := bytes.NewBufferString("[")
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return nil, err
		}
		if _, err = buf.Write(kv.Value); err != nil {
			return nil, err
		}
		if iter.HasNext() {
			if err = buf.WriteByte(','); err != nil {
				return nil, err
			}
		}
	}
	if err := buf.WriteByte(']'); err != nil {
		return nil, err
	}

	result.Records = buf.Bytes()

	return result, nil
}

// MarshalJSON _
func (qr *QueryResult) MarshalJSON() ([]byte, error) {
	var err error
	buf := bytes.NewBufferString("{")
	if qr.Meta != nil {
		if _, err = buf.WriteString(`"meta":`); err != nil {
			return nil, err
		}
		meta, err := json.Marshal(qr.Meta)
		if err != nil {
			return nil, err
		}
		if _, err = buf.Write(meta); err != nil {
			return nil, err
		}
		if err = buf.WriteByte(','); err != nil {
			return nil, err
		}
	}
	if _, err = buf.WriteString(`"records":`); err != nil {
		return nil, err
	}
	if _, err = buf.Write(qr.Records); err != nil {
		return nil, err
	}
	if err = buf.WriteByte('}'); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
