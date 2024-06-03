# extend

[中文](README-CN.md)

Filecoin Sector Renewal Service

[![Build](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml/badge.svg)](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml)

## Deployment
```shell
# Run tests
make test

# Build
make  # or make docker

# Run
export FULLNODE_API_INFO="lotus api info"  # lotus api info, need sign permission

./extend run   
OPTIONS:  
 --listen value  specify the address to listen on (default: "127.0.0.1:8000")
 --db value      specify the database file to use (default: "extend.db") 
 --secret value  specify the secret to use for API authentication, if not set, no auth will be enabled 
 --max-wait value  [Warning] specify the maximum time to wait for messages on chain, otherwise try to replace them, only use this if you know what you are doing (default: 0s)
 --debug         enable debug logging (default: false) 
 --help, -h      show help  

# View help
./extend -h  
```
### Docker

```shell
docker run -d --name extend \
  -p 8000:8000 
  -e FULLNODE_API_INFO="lotus api info" 
  -v /path/to/extend:/app
  gh-efforts/extend:latest run --listen 0.0.0.0:8000 --db /app/extend.db
```

### Enable authentication (optional)

If you start the service with `--secret yoursecret`, the API will require authentication; otherwise, it will not.

Generate a token:
```shell
./extend auth create-token --secret yoursecret --user lee [--expiry optional_expiry_time]

Token created successfully:
eyJhbGciOiJIUzI1NiIsInR5cI6IkpXVCJ9.eyJzdWIiOiJsZWUiLJuYmYiOjE3MTU1zExMzgsImlhdCI6MTcxNTU3MTEzOH0.2FinpGB7D07dLNWPAW3jqo703i13nZU8ZCY__wKnQ
```

Use the token:
```
Authorization: Bearer  <token>
```

Changing `--secret yoursecret` will invalidate all tokens generated based on the old secret.

## Usage

### POST /requests
Create a renewal request.

#### Request parameters
| Parameter      | Value                        | Required | Description                                                                                                                                             |  
|----------------|------------------------------|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------|  
| miner          | f01234                       | Yes      | Miner ID                                                                                                                                                |  
| from           | 2024-05-12T07:34:47+00:00    | Yes      | Start time of the sector expiration range in RFC3339 format; renew sectors expiring within this range                                                   |  
| to             | 2024-05-13T07:34:47+00:00    | Yes      | End time of the sector expiration range in RFC3339 format; renew sectors expiring within this range                                                     |  
| extension      | 20160                        | No       | Number of epochs to extend; adds this period to the original expiration                                                                                 |  
| new_expiration | 3212223                      | No       | Extend to a specified epoch, overriding the previous expiration; if set, `extension` is ignored                                                         |  
| tolerance      | 20160, 7 days, default value | No       | Renewal tolerance for accuracy; aggregates sectors with close expiration times to reduce message size and gas costs; ignored if `new_expiration` is set |  
| max_sectors    | 500, default value           | No       | Maximum number of sectors allowed per message, up to 25000;                                                                                             |  
| dry_run        | false, default               | No       | Test mode; if true, no actual renewal operation is performed, only checks are conducted                                                                 |  

#### Request example
```shell
{  
    "miner": "f01234", 
    "from": "2024-05-12T07:34:47+00:00", 
    "to": "2024-05-13T07:34:47+00:00", 
    "extension": 20160, 
    "new_expiration": 3212223, 
    "tolerance": 20160, 
    "max_sectors": 500,
    "dry_run": false
}  
```
#### Response parameters
| Parameter         | Value                                                 | Description                                                    |  
|-------------------|-------------------------------------------------------|----------------------------------------------------------------|  
| id                | 11                                                    | Request ID                                                     |  
| extension         | 21000                                                 | Extension height specified during creation                     |  
| from              | "2024-05-12T07:34:47Z"                                | Start time of the sector expiration range                      |  
| to                | "2024-05-13T07:34:47Z"                                | End time of the sector expiration range                        |  
| new_expiration    | null                                                  | New expiration time specified during creation                  |  
| max_sectors       | 500                                                   | Maximum number of sectors per message                          |
| messages          | null                                                  | Renewal messages on-chain, array                               |  
| tolerance         | 20160                                                 | Renewal tolerance specified during creation                    |  
| miner             | "t017387"                                             | Miner                                                          |  
| status            | "failed"                                              | Status: `created`, `pending`, `failed`, `partfailed`,`success` |  
| took              | 526.841994321                                         | Execution time in seconds                                      |  
| confirmed_at      | null                                                  | Confirmation time of the message on-chain                      |  
| dry_run           | true                                                  | Indicates if it is a test run                                  |  
| dry_run_result    | ""                                                    | Test run result                                                |  
| error             | "failed to get active sector set: RPCConnectionError" | Error message                                                  |  
| total_sectors     | 1000                                                  | Number of sectors to be renewed                                |  
| published_sectors | 500                                                   | Number of sectors actually published on-chain                  |
| succeeded_sectors | 0                                                     | Number of sectors successfully renewed                         |
| created_at        | "2024-05-11T13:39:40.74831759+08:00"                  | Creation time                                                  |  
| updated_at        | "2024-05-11T13:40:16.237069667+08:00"                 | Update time                                                    |  

#### Response Example
```json
{  
  "data": { 
    "confirmed_at": null, 
    "created_at": "2024-05-11T13:39:40.74831759+08:00", 
    "dry_run": true, 
    "dry_run_result": "", 
    "error": "failed to get active sector set: RPCConnectionError", 
    "extension": 21000, 
    "tolerance": 20160, 
    "max_sectors": 500,
    "from": "2024-05-12T07:34:47Z", 
    "id": 11, 
    "messages": null, 
    "miner": "t017387", 
    "new_expiration": null, 
    "status": "created", 
    "to": "2024-05-13T07:34:47Z", 
    "total_sectors": 0,
    "published_sectors": 0,
    "succeeded_sectors": 0,
    "took": 526.841994321, 
    "updated_at": "2024-05-11T13:40:16.237069667+08:00"
  }
}
```

### GET /requests/{:id｝
Query a renewal request by ID, returning the same structure as when created.

##### Response Example
```
{
  "data": {
    "confirmed_at": null,
    "created_at": "2024-05-13T16:44:58.208723388+08:00",
    "dry_run": false,
    "dry_run_result": "",
    "error": "",
    "extension": 21000,
    "from": "2024-05-13T15:18:47+08:00",
    "id": 19,
    "messages": [
      "bafy2bzacecvqxeqlk4z2gugbtbhgjmptdfjpt73jemf2zwtbj7lepxhf4h6r4"
    ],
    "miner": "f017387",
    "new_expiration": null,
    "max_sectors": 500,
    "status": "pending",
    "to": "2024-05-14T15:18:47+08:00",
    "total_sectors": 1000,
    "published_sectors": 500,
    "succeeded_sectors": 0,
    "took": 526.841994321,
    "updated_at": "2024-05-13T16:53:55.942614308+08:00"
  }
}
```

### POST /requests/{:id}/speedup
Accelerate a renewal request by re-estimating the gas for all messages that have not been confirmed on-chain and are in a pending state.

> [!WARNING]
> Only requests in the pending state can be accelerated. Requests in other states cannot be accelerated.
> Speedup returns success but does not guarantee that the message will be on-chain. Check the request status again after a while and try multiple times if needed.

#### Request parameters
| Parameter | Value | Required | Description                                                                                                          |  
|-----------|-------|----------|----------------------------------------------------------------------------------------------------------------------|  
| fee_limit | 1FIL  | No       | Maximum allowable gas fee; if not provided, the system estimates; the higher the value, the faster the on-chain time |  

#### Request Example
 ```json
 {
  "fee_limit": "1FIL"
}
```

#### Response Example
```json
 {
  "data": "success"
}
```

## Exceptions
HTTP Status Codes:

* 400: Bad Request
* 500: Internal Server Error
* 401: Unauthorized (if authentication is enabled)

```json
{
  "error": "msg"
}
```