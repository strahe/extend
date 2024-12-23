# extend

[English](README.md)

Filecoin sector 续期服务

[![Build](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml/badge.svg)](https://github.com/gh-efforts/extend/actions/workflows/container-build.yml)

## 部署

### 运行测试
```shell
make test
```

### 编译
```shell
make  # 或者 make docker
```

### 运行
在运行之前，请确保设置了 `FULLNODE_API_INFO` 环境变量，具有必要的权限:
```shell
export FULLNODE_API_INFO="lotus api info"  # 需要 'sign' 权限
```

启动服务:
```shell
./extend run

OPTIONS:
   --listen value    specify the address to listen on (default: "127.0.0.1:8000")
   --db value        specify the database URL to use， support sqlite3, mysql, postgres, https://github.com/xo/dburl?tab=readme-ov-file#example-urls (default: "sqlite3:extend.db")
   --secret value    specify the secret to use for API authentication, if not set, no auth will be enabled
   --max-wait value  [Warning] specify the maximum time to wait for messages on chain, otherwise try to replace them, only use this if you know what you are doing (default: 0s)
   --debug           enable debug logging (default: false)
   --help, -h        show help
```

查看帮助:
```shell
./extend -h  
```

### Docker
使用Docker运行服务:
```shell
docker run -d --name extend \
  -p 8000:8000 \
  -e FULLNODE_API_INFO="lotus api info" \
  -v /path/to/extend:/app \
  ghcr.io/strahe/extend:latest run --listen 0.0.0.0:8000 --db /app/extend.db
```

### 开启鉴权 (可选)
通过使用 `--secret` 选项启用鉴权:
```shell
./extend run --secret yoursecret
```

#### 生成token
```shell
./extend auth create-token --secret yoursecret --user {user} [--expiry 可选过期时间]

# Example output
token created successfully:
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJsZWUiLCJuYmYiOjE3MTU1NzExMzgsImlhdCI6MTcxNTU3MTEzOH0.2FinpGB7Dx07dLNWPAW3jqo703i13nZUB8ZCY__wKnQ
```
使用token:
```
Authorization:  Bearer  <token>
```
> [!Warning]
> 修改 `--secret yoursecret` ， 基于secret生成的所有token将会失效.

## 2. Usage

### 创建一个续期请求

```http request
POST /requests
```

#### 请求参数

| 参数名                 | 值                         | 是否必须 | 说明                                                                                       |  
|---------------------|---------------------------|------|------------------------------------------------------------------------------------------|  
| miner               | f01234                    | 是    | 节点名字                                                                                     |  
| from                | 2024-05-12T07:34:47+00:00 | 是    | sector到期范围的开始时间,RFC3339格式，表明续期这个范围的sector                                                |  
| to                  | 2024-05-13T07:34:47+00:00 | 是    | sector到期的结束时间,RFC3339格式，表明续期这个范围的sector                                                  |  
| extension           | 20160                     | 否    | 续期的Epoch数量，即在原有效期的基础上追加对应的高度                                                             |  
| new_expiration      | 3212223                   | 否    | 不管之前的有效期是多少，统一续期到指定的高度，如果设置了这个值，extension将被忽略                                            |  
| tolerance           | 20160，7天，默认值              | 否    | 续期公差，精度，将到期时间相近的sector聚合成相同的到期时间，减少消息大小，降低gas, 且最低续期时间不能低于这个值， 使用new_expiration时，这个值会被忽略 |
| max_sectors         | 500, 默认值                  | 否    | 单条消息允许包含的最大sector数量，最大25000                                                              |
| max_initial_pledges | 0, 默认值                    | 否    | 续期的扇区中，累计允许的最大质押值，如果质押币累计超过设置的值，其余的sector将不会被续期，默认不限制                                    |
| basefee_limit       | null                      | No   | 只有当网络basefee低于这个值时才发送续期消息， 0 和 null 表示不限制                                                |
| dry_run             | false，默认                  | 否    | 是否是测试，如果是测试，不会真正执行续期操作，只会做一些检查                                                           |  

#### 请求示例
```json  
{  
    "miner": "f01234", 
    "from": "2024-05-12T07:34:47+00:00", 
    "to": "2024-05-13T07:34:47+00:00", 
    "extension": 20160, 
    "new_expiration": 3212223, 
    "tolerance": 20160, 
    "max_sectors": 500,
    "basefee_limit": null,
    "dry_run": false
 }  
```  

#### 返回参数
| 参数名                 | 值                                                     | 说明                                                     |  
|---------------------|-------------------------------------------------------|--------------------------------------------------------|  
| id                  | 11                                                    | 请求ID                                                   |  
| extension           | 21000                                                 | 续期追加的高度，创建时指定的参数                                       |  
| from                | "2024-05-12T07:34:47Z"                                | 筛选过期sector的开始时间，创建时指定的参数                               |  
| to                  | "2024-05-13T07:34:47Z"                                | 筛选过期sector的结束时间，创建时指定的参数                               |  
| new_expiration      | null                                                  | 新的过期时间，创建时指定的参数                                        |  
| max_sectors         | 500                                                   | 单条消息允许包含的最大sector数量，创建时指定的参数                           |
| basefee_limit       | null                                                  | 只有当网络basefee低于这个值时才发送续期消息                              |
| max_initial_pledges | 0                                                     | 续期的扇区中，累计允许的最大质押值，如果质押币累计超过设置的值，其余的sector将不会被续期，默认不限制  |
| messages            | null                                                  | 续期上链的消息，array                                          |  
| tolerance           | 20160                                                 | 续期公差，创建时指定的参数                                          |
| miner               | "t017387"                                             | 矿工                                                     |  
| status              | "failed"                                              | 状态，`created`,`pending`,`failed`,`partfailed`,`success` |  
| took                | 526.841994321                                         | 续期执行耗时,单位s                                             |  
| confirmed_at        | null                                                  | 消息上链的确认时间                                              |  
| dry_run             | true                                                  | 是否为测试运行                                                |  
| dry_run_result      | ""                                                    | 测试运行结果                                                 |  
| error               | "failed to get active sector set: RPCConnectionError" | 错误信息                                                   |  
| total_sectors       | 1000                                                  | 续期的sector数量                                            |
| published_sectors   | 500                                                   | 实际上链的sector数量，注：上链并不一定成功                               |
| succeeded_sectors   | 0                                                     | 成功续期的sector数量                                          |
| created_at          | "2024-05-11T13:39:40.74831759+08:00"                  | 创建时间                                                   |  
| updated_at          | "2024-05-11T13:40:16.237069667+08:00"                 | 更新时间                                                   |  

#### 返回示例
```json  
{  
  "data": {
    "basefee_limit": null,
    "confirmed_at": null, 
    "created_at": "2024-05-11T13:39:40.74831759+08:00", 
    "dry_run": true, 
    "dry_run_result": "", 
    "error": "failed to get active sector set: RPCConnectionError", 
    "extension": 21000, 
    "tolerance": 20160, 
    "max_sectors": 500,
    "max_initial_pledges": 0,
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

### 查询续期请求
根据ID查询一个续期请求，返回的结构与创建时一致
```http request
GET /requests/{:id}
```

#### 返回示例
 ```json
{
  "data": {
    "basefee_limit": null,
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
    "max_initial_pledges": 0,
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

### 加速续期请求
加速一个续期请求，将一个pending状态的请求未上链的所有消息重新预估gas后上链.
```http request
POST /requests/{:id}/speedup
```
> [!WARNING]
> 只有pending状态的请求才能加速，其他状态的请求不能加速.
> 加速返回success，并不能保证消息一定会上链，等待一段时间后再次查询请求状态，可多次尝试.

#### 请求参数

| 参数名       | 值    | 是否必须 | 说明                    |  
|-----------|------|------|-----------------------|  
| fee_limit | 1FIL | 否    | 允许最大消耗的GAS费用,不传系统自动预估 |  

#### 请求示例
 ```json
 {
  "fee_limit": "1FIL"
}
```

#### 返回示例
 ```json
 {
  "data": "success"
}
```

### Update Request
更新一个续期请求，只有未开始的请求才能更新， 目前只支持更新 `basefee_limit` 参数.

```http request
PATCH /requests/{:id}
```

#### Request Example
> [!NOTE]
> 0 和 null 表示不限制

 ```json
 {
  "basefee_limit": 0 
}
```

#### Response Example
和创建请求的返回一致

### 使用限制

续期有很多硬性和软性的限制， 其中硬性限制是无法绕过的，包括：
* 每个请求的sector数量不能超过25000个
* 每个请求最大的Declarations不能超过3000个
* sector 只能最大生命周期是5年
* sector 最大续期周期是3年半 （1278d）

软性限制包括：
* gas fee 限制
* 虚拟机性能限制等

> [!TIP]
> 什么是 Declarations？
> 想象一个二维坐标轴，x轴是要续期的目的高度，y轴是sector的位置(deadline,partition),任何续期的sector都可以用一个坐标来表示.
> sector 的位置(y)和续期目的高度(x)可以相同，即在坐标上用相同的点表示，也可以不同，即在坐标上用不同的点表示.
> 这一个点就是一个Declaration，一个请求可以有多个Declaration，但是不能超过3000个.
> 一个Declaration可以包含多个sector，且所有Declarations包含的sector不能大于25000个.

> [!TIP]
> 什么是 Tolerance? 
> 根据Declarations的定义可以得知，续期限制主要是Declaration的数量，即二维坐标上的点的数量，而不是sector的数量.
> Tolerance是一个续期公差，用于将续期后的到期时间相近的sector聚合成相同的到期时间，即相同的x坐标，
> 这样在二维坐标上有更多的点重合，降低Declaration数量，从而降低消息大小，降低gas fee.
> 同时要求最低续期时间不能低于这个值.
> 举例说明： 如果一个sector续期到 2024-06-01:00:00:00, 另一个sector续期到它的6小时以后，即2024-06-01:00:06:00， 此时如果我们设置的Tolerance公差大于6小时，那么第二个sector也会续期到和第一个sector相同的时间，即2024-06-01:00:00:00，（比实际少续6小时）， 如果我们设置Tolerance公差小于6小时，那么这两个sector会单独续期到各自的时间.

## 异常

Http status：
* 400： 请求问题
* 500： 服务器问题
* 401： 授权问题（如果开启）

```json
{
  "error": "msg"
}
```
