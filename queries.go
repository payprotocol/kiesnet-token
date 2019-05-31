// Copyright Key Inside Co., Ltd. 2018 All Rights Reserved.

package main

import (
	"fmt"

	"github.com/key-inside/kiesnet-ccpkg/txtime"
)

// QueryBalanceLogsByID _
const QueryBalanceLogsByID = `{
	"selector": {
		"@balance_log": "%s"
		%s
	},
	"sort": [%s],
	"use_index": ["balance", "%s"]
}`

// CreateQueryBalanceLogsByID _
func CreateQueryBalanceLogsByID(id, typeStr string) string {
	_type := ""
	_sort := `{"@balance_log": "desc"}, {"created_time": "desc"}`
	_index := "logs"
	if typeStr != "" {
		_type = fmt.Sprintf(`,"type":%s`, typeStr)
		_sort = `{"@balance_log": "desc"}, {"type": "desc"}, {"created_time": "desc"}`
		_index = "logs-type"
	}
	return fmt.Sprintf(QueryBalanceLogsByID, id, _type, _sort, _index)
}

// QueryBalanceLogsByIDAndTimes _
const QueryBalanceLogsByIDAndTimes = `{
	"selector": {
		"@balance_log": "%s",
		%s
		"$and": [
            {
                "created_time": {
                    "$gte": "%s"
                }
            },
            {
                "created_time": {
                    "$lt": "%s"
                }
            }
        ]
	},
	"sort": [%s],
	"use_index": ["balance", "%s"]
}`

// CreateQueryBalanceLogsByIDAndTimes _
func CreateQueryBalanceLogsByIDAndTimes(id, typeStr string, stime, etime *txtime.Time) string {
	_type := ""
	_sort := `{"@balance_log": "desc"}, {"created_time": "desc"}`
	_index := "logs"
	if typeStr != "" {
		_type = fmt.Sprintf(`"type":%s,`, typeStr)
		_sort = `{"@balance_log": "desc"}, {"type": "desc"}, {"created_time": "desc"}`
		_index = "logs-type"
	}
	if nil == stime {
		stime = txtime.Unix(0, 0)
	}
	if nil == etime {
		etime = txtime.Unix(253402300799, 999999999) // 9999-12-31 23:59:59.999999999
	}
	return fmt.Sprintf(QueryBalanceLogsByIDAndTimes, id, _type, stime.String(), etime.String(), _sort, _index)
}

// QueryHoldersByID _
const QueryHoldersByID = `{
	"selector": {
		"@holder": "%s"
	},
	"sort": ["@holder", "token", "type"],
	"use_index": ["account", "holder"]
}`

// CreateQueryHoldersByID _
func CreateQueryHoldersByID(id string) string {
	return fmt.Sprintf(QueryHoldersByID, id)
}

// QueryHoldersByIDAndTokenCode _
const QueryHoldersByIDAndTokenCode = `{
	"selector": {
		"@holder": "%s",
		"token": "%s"
	},
	"sort": ["@holder", "token", "type"],
	"use_index": ["account", "holder"]
}`

// CreateQueryHoldersByIDAndTokenCode _
func CreateQueryHoldersByIDAndTokenCode(id, tokenCode string) string {
	return fmt.Sprintf(QueryHoldersByIDAndTokenCode, id, tokenCode)
}

// QueryPendingBalancesByAddress _
const QueryPendingBalancesByAddress = `{
	"selector": {
		"@pending_balance": {
			"$exists": true
		},
		"account": "%s"
	},
	"sort": [%s],
	"use_index": "pending-balance"
}`

// CreateQueryPendingBalancesByAddress _
func CreateQueryPendingBalancesByAddress(addr, sort string) string {
	var _sort string
	if "created_time" == sort {
		_sort = `{"account":"desc"},{"created_time":"desc"}`
	} else { // pending_time
		_sort = `"account","pending_time"`
	}
	return fmt.Sprintf(QueryPendingBalancesByAddress, addr, _sort)
}

// QueryPrunePays _
const QueryPrunePays = `{
	"selector":{		
		"@pay": "%s",
		"$and":[
			{
				"created_time":{
					"$gt": "%s"
				}
			},{
				"created_time":{
					"$lte": "%s"
				}

			}
		] 
	},
	"use_index":["pay","list"]
}`

// CreateQueryPrunePays _
func CreateQueryPrunePays(id string, stime, etime *txtime.Time) string {
	return fmt.Sprintf(QueryPrunePays, id, stime, etime)
}

// QueryPaysByIDAndTime _
const QueryPaysByIDAndTime = `{
	"selector":{
		"@pay":"%s",
		"$and":[
			{
				"created_time":{
					"$gte": "%s"
				}
			},
			{
				"created_time":{
					"$lte": "%s"
				}
			}
		]
	},
	"use_index":["pay","list"],
	"sort":[{"created_time":"%s"}]
}`

// CreateQueryPaysByIDAndTime _
func CreateQueryPaysByIDAndTime(id, sortOrder string, stime, etime *txtime.Time) string {
	if nil == stime {
		stime = txtime.Unix(0, 0)
	}
	if nil == etime {
		etime = txtime.Unix(253402300799, 999999999) // 9999-12-31 23:59:59.999999999
	}
	return fmt.Sprintf(QueryPaysByIDAndTime, id, stime, etime, sortOrder)
}

// QueryPaysByID _
const QueryPaysByID = `{
	"selector":{
		"@pay":"%s"
	},
	"use_index":["pay","list"],
	"sort":[{"@pay":"%s"},{"created_time":"%s"}]
}`

// CreateQueryPaysByID _
func CreateQueryPaysByID(id, sortOrder string) string {
	return fmt.Sprintf(QueryPaysByID, id, sortOrder, sortOrder)
}

// QueryPayByOrderID _
const QueryPayByOrderID = `{
	"selector":{
		"order_id":"%s"
	},
	"sort":["order_id"],
	"use_index":["pay","order-id"]
}`

// CreateQueryPayByOrderID _
func CreateQueryPayByOrderID(orderID string) string {
	return fmt.Sprintf(QueryPayByOrderID, orderID)
}

// QueryPruneFee _
//TODO check sort, use_index
const QueryPruneFee = `{
	"selector":{
		"@fee":"%s",
		"created_time":{"$gt":"%s"},
		"created_time":{"$lte":"%s"}
	},
	"sort":["created_time"],
	"use_index":["fee","list"]
}`

// CreateQueryPruneFee generates query string to fetch fee list of tokenCode from stime to etime.
func CreateQueryPruneFee(tokenCode string, stime, etime *txtime.Time) string {
	return fmt.Sprintf(QueryPruneFee, tokenCode, stime, etime)
}

// QueryFeesByCodeAndTime _
const QueryFeesByCodeAndTime = `{
	"selector":{
		"@fee":"%s",
		"$and":[
			{"created_time":{"$gte":"%s"}},
			{"created_time":{"$lte":"%s"}}
		]
	},
	"sort":[{"@fee":"desc"},{"created_time":"desc"}],
	"use_index":["fee","list"]
}`

// CreateQueryFeesByCodeAndTimes _
func CreateQueryFeesByCodeAndTimes(tokenCode string, stime, etime *txtime.Time) string {
	if nil == stime {
		stime = txtime.Unix(0, 0)
	}
	if nil == etime {
		etime = txtime.Unix(253402300799, 999999999) // 9999-12-31 23:59:59.999999999
	}
	return fmt.Sprintf(QueryFeesByCodeAndTime, tokenCode, stime, etime)
}

// QueryFeesByCode _
const QueryFeesByCode = `{
	"selector":{
		"@fee":"%s"
	},
	"sort":[{"@fee":"desc"},{"created_time":"desc"}],
	"use_index":["fee","list"]
}`

// CreateQueryFeesByCode _
func CreateQueryFeesByCode(tokenCode string) string {
	return fmt.Sprintf(QueryFeesByCode, tokenCode)
}
