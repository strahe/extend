# extend

[中文](README-CN.md)

Filecoin Sector Renewal Service

[![Build](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml/badge.svg)](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml)


## Deployment

### Running Tests
```shell
make test
```

### Building
```shell
make  # or make docker
```

### Running
Before running, ensure you set the `FULLNODE_API_INFO` environment variable with the necessary permissions:
```shell
export FULLNODE_API_INFO="lotus api info"  # Requires 'sign' permission
```

To start the service:
```shell
./extend run   

OPTIONS:  
 --listen value   Specify the address to listen on (default: "127.0.0.1:8000")
 --db value       Specify the database file to use (default: "extend.db") 
 --secret value   Specify the secret to use for API authentication; if not set, no auth will be enabled 
 --max-wait value [Warning] Specify the maximum time to wait for messages on-chain; attempts replacement if exceeded. Use with caution (default: 0s)
 --debug          Enable debug logging (default: false) 
 --help, -h       Show help  
```
Get help
```shell
./extend -h  
```

### Docker
Run the service using Docker:
```shell
docker run -d --name extend \
  -p 8000:8000 
  -e FULLNODE_API_INFO="lotus api info" 
  -v /path/to/extend:/app
  gh-efforts/extend:latest run --listen 0.0.0.0:8000 --db /app/extend.db
```

### Enable authentication (optional)
Enable authentication by starting the service with the `--secret` option:
```shell
./extend run --secret yoursecret
```

#### Generating Tokens
Generate a token for API access:
```shell
./extend auth create-token --secret yoursecret --user {name} [--expiry optional_expiry_time]

# Example output
Token created successfully:
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJsZWUiLJuYmYiOjE3MTU1zExMzgsImlhdCI6MTcxNTU3MTEzOH0.2FinpGB7D07dLNWPAW3jqo703i13nZU8ZCY__wKnQ
```
Use the token in your requests:
```
Authorization: Bearer  <token>
```
> [!WARNING] 
> Changing `--secret yoursecret` will invalidate all previously generated tokens.

## Usage

### Creating Renewal Requests
Create a renewal request by sending a POST request to `/requests`:
```http request
POST /requests
```

#### Request parameters
| Parameter      | Value                        | Required | Description                                                                           |  
|----------------|------------------------------|----------|---------------------------------------------------------------------------------------|  
| miner          | f01234                       | Yes      | Miner ID                                                                              |  
| from           | 2024-05-12T07:34:47+00:00    | Yes      | Start time of the sector expiration range in RFC3339 format                           |  
| to             | 2024-05-13T07:34:47+00:00    | Yes      | End time of the sector expiration range in RFC3339 format                             |  
| extension      | 20160                        | No       | Number of epochs to extend                                                            |  
| new_expiration | 3212223                      | No       | New expiration epoch, overrides `extension`                                           |  
| tolerance      | 20160, 7 days, default value | No       | Renewal tolerance for aggregation                                                     |  
| max_sectors    | 500, default value           | No       | Maximum sectors per message, up to 25000                                              |  
| dry_run        | false, default               | No       | If true, performs a test run without actual renewal                                   |  

#### Request example
```json
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

### Querying Renewal Requests
Query a renewal request by its ID:
```http request
GET /requests/{:id}
```
The response structure is the same as the creation response.

##### Response Example
```json
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

### Speeding Up Requests
Accelerate a renewal request by re-estimating the gas for all messages that are still pending on-chain:
```http request
POST /requests/{:id}/speedup
```
> [!WARNING]
> Only pending requests can be accelerated. 
> This does not guarantee the message will be on-chain. Check the request status again after a while and try multiple times if needed.

#### Request parameters
| Parameter | Value | Required | Description                                                       |  
|-----------|-------|----------|-------------------------------------------------------------------|  
| fee_limit | 1FIL  | No       | Maximum allowable gas fee; if not provided, the system estimates; |  

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