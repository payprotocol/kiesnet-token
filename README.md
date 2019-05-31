# Kiesnet Token Chaincode

Hybrid (account/balance + UTXO) token

#

## Terms

- PAOT : personal(main) account of the token

#

## API

method __`func`__ [arg1, _arg2_, ... ] {trs1, _trs2_, ... }
- method : __query__ or __invoke__
- func : function name
- [arg] : mandatory argument
- [_arg_] : optional argument
- {trs} : mandatory transient
- {_trs_} : optional transient

#

> invoke __`account/create`__ [token_code, _co-holders..._] {_"kiesnet-id/pin"_}
- Create an account
- [token_code] : issued token code
- [_co-holders..._] : PAOTs (exclude invoker, max 127)
- If holders(include invoker) are more then 1, it creates a joint account. If not, it creates the PAOT.

> query __`account/get`__ [token_code|address]
- Get the account
- If the parameter is token code, it returns the PAOT.
- account types
    - 0x00 : unknown
    - 0x01 : personal
    - 0x02 : joint

> invoke __`account/holder/add`__ [account, holder] {_"kiesnet-id/pin"_}
- Create a contract to add the holder
- [account] : the joint account address
- [holder] : PAOT of the holder to be added

> invoke __`account/holder/remove`__ [account, holder] {_"kiesnet-id/pin"_}
- Create a contract to remove the holder
- [account] : the joint account address
- [holder] : PAOT of the holder to be removed

> query __`account/list`__ [token_code, _bookmark_, _fetch_size_]
- Get account list
- If token_code is empty, it returns all account regardless of tokens.
- [_fetch_size_] : max 200, if it is less than 1, default size will be used (20)

> invoke __`account/suspend`__ [token_code] {_"kiesnet-id/pin"_}
- Suspend the PAOT

> invoke __`account/unsuspend`__ [token_code] {_"kiesnet-id/pin"_}
- Unsuspend the PAOT

> query __`balance/logs`__ [token_code|address, _log_type_, _bookmark_, _fetch_size_, _starttime_, _endtime_]
- Get balance logs
- If the parameter is token code, it returns logs of the PAOT.
- [_fetch_size_] : max 200, if it is less than 1, default size will be used (20)
- [_starttime_] : __time(seconds)__ represented by int64
- [_endtime_] : __time(seconds)__ represented by int64
- log types
    - 0x00 : mint
    - 0x01 : burn
    - 0x02 : send
    - 0x03 : receive
    - 0x04 : deposit (create a pending balance)
    - 0x05 : withdraw (from the pending balance)
    - 0x06 : pay
    - 0x07 : refund
    - 0x08 : prune pay
    - 0x09 : prune fee

> query __`balance/pending/get`__ [pending_balance_id]
- Get the pending balance
- pending types
    - 0x00 : account
    - 0x01 : contract

> query __`balance/pending/list`__ [token_code|address, _sort_, _bookmark_, _fetch_size_]
- Get pending balances list
- If the parameter is token code, it returns logs of the PAOT.
- [_sort_] : 'created_time' or 'pending_time'(default)
- [_fetch_size_] : max 200, if it is less than 1, default size will be used (20)
- pending types
    - 0x00 : account
    - 0x01 : contract

> invoke __`balance/pending/withdraw`__ [pending_balance_id] {_"kiesnet-id/pin"_}
- Withdraw the balance

> query __`fee/list`__ [token_code, _bookmark_, _fetch_size_, _starttime_, _endtime_]
- Get fee list of token
- [_fetch_size_] : max 200, if it is less than 1, default size will be used (20)
- [_starttime_] : __time(seconds)__ represented by int64
- [_endtime_] : __time(seconds)__ represented by int64

> invoke __`fee/prune`__ [token_code, ten_minutes_flag, _endtime_] {_"kiesnet-id/pin"_}
- prune the fees from last fee time to end_time. if end_time is not provided, prune to 10 mins lesser than current time(if ten_minutes_flag is set to true).
- Only holder of FeePolicy.TargetAddress is able to prune.
- [ten_minutes_flag] : __Boolean__ if set to true, the end_time can't be greater than current time minus 10 minutes.
- [_end_time_]: to time for pruning
- __`has_more`__ field is __true__ in the response json string, it means there are more fees to prune given time period.

> invoke __`token/burn`__ [token_code, amount] {_"kiesnet-id/pin"_}
- Get the burnable amount and burn the amount.
- [amount] : big int
- If genesis account holders are more than 1, it creates a contract.

> invoke __`token/create`__ [token_code, _co-holders..._] {_"kiesnet-id/pin"_}
- Create(Issue) the token
- [token_code] : 3~6 alphanum
- [_co-holders..._] : PAOTs (exclude invoker, max 127)
- It queries meta-data of the token from the knt-{token_code} chaincode.

> query __`token/get`__ [token_code]
- Get the current state of the token

> invoke __`token/mint`__ [token_code, amount] {_"kiesnet-id/pin"_}
- Get the mintable amount and mint the amount.
- [amount] : big int
- If genesis account holders are more than 1, it creates a contract.

> invoke __`token/update`__ [token_code] {_"kiesnet-id/pin"_}
- // Get updated information from the token meta chaincode(e.g. knt-cc-pci) and save it to the ledger.
- [token_code] : issued token code. If the token is not issued, this function does nothing and returns success.

> invoke __`transfer`__ [sender, receiver, amount, _memo_, _pending_time_, _expiry_, _extra-signers..._] {_"kiesnet-id/pin"_}
- Transfer the amount of the token or create a contract
- [sender] : an account address, __empty = PAOT__
- [receiver] : an account address
- [amount] : big int
- [_memo_] : max 1024 charactors
- [_pending_time_] : __time(seconds)__ represented by int64
- [_expiry_] : __duration(seconds)__ represented by int64, multi-sig only
- [_extra-signers..._] : PAOTs (exclude invoker, max 127)

> invoke __`pay`__ [sender, receiver, amount(+), _memo_, _expiry_] {_"kiesnet-id/pin"_}
- pay the amount of **positive** token to the receiver or creaete a pay contract
- [sender]: an account address, __TOKENCODE = PAOT__
- [receiver] : an account address
- [amount] : big int(+)
- [_memo_] : max 1024 charactors
- [_expiry_] : __duration(seconds)__ represented by int64, multi-sig only

> invoke __`pay/refund`__ [original_pay_key, amount(+), _memo_ ] {_"kiesnet-id/pin"_}
- refund the amount of token the based on original_pay_key 
- [original_pay_key] : original_pay_key 
- [amount]: the amount of token. this value should be lesser than original pay's amount
- [_memo_]: max 1024 charactors

> invoke __`pay/prune`__ [token_code|address, ten_minutes_flag, _end_time_] {_"kiesnet-id/pin"_}
- prune the pays from last pay time to end_time. if end_time is not provided, prune to 10 mins lesser than current time(if ten_minutes_flag is set to true).
- [ten_minutes_flag] : __Boolean__ if set to true, the end_time can't be greater than current time minus 10 minutes.
- [_end_time_]: to time for pruning
- __`has_more`__ field is __true__ in the response json string, it means there are more pays to prune given time period.

> query __`pay/list`__ [token_code|address, sort_order, _bookmark_, _fetchsize_, _start_time_, _end_time_ ]
- Get pay list
- If the 1st parameter is token code, it returns list of the PAOT.
- [_sort_order_] : "asc" ascending order. "desc" decending order. if not set, decending order is the default value.
- [_fetch_size_] : max 200, if it is less than 1, default size will be used (20)
- [_starttime_] : __time(seconds)__ represented by int64
- [_endtime_] : __time(seconds)__ represented by int64

> query __`ver`__
- Get version
