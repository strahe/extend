
# extend

Filecoin sector 续期服务

## 部署

```shell  
# 编译主网程序  
make extend  
  
# 编译测试网程序  
make calibnet  
  
# 运行  
./extend run   
OPTIONS:  
 --listen value  specify the address to listen on (default: "127.0.0.1:8000")
 --db value      specify the database file to use (default: "extend.db") 
 --secret value  specify the secret to use for API authentication, if not set, no auth will be enabled 
 --debug         enable debug logging (default: false) --help, -h      show help  

# 查看帮助  
./extend -h  
```
### 开启鉴权 (可选)
如果启动服务时指定了 `--secret yoursecret`, 那么API接口将开启鉴权，否则不开启。
开启鉴权之后，所有api请求需要token.

生成token:
```shell
./extend auth create-token --secret yoursecret --user lee [--expiry 可选过期时间]

token created successfully:
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJsZWUiLCJuYmYiOjE3MTU1NzExMzgsImlhdCI6MTcxNTU3MTEzOH0.2FinpGB7Dx07dLNWPAW3jqo703i13nZUB8ZCY__wKnQ
```
使用token:
```
Authorization:  Bearer  <token>
```
修改 `--secret yoursecret` ， 基于secret生成的所有token将会失效.

## 2. Usage

### POST /requests/create
创建一个续期请求
#### 请求参数

| 参数名            | 值                         | 是否必须 | 说明                                            |  
|----------------|---------------------------|------|-----------------------------------------------|  
| miner          | f01234                    | 是    | 节点名字                                          |  
| from           | 2024-05-12T07:34:47+00:00 | 是    | sector到期范围的开始时间,RFC3339格式，表明续期这个范围的sector     |  
| to             | 2024-05-13T07:34:47+00:00 | 是    | sector到期的结束时间,RFC3339格式，表明续期这个范围的sector       |  
| extension      | 20160                     | 否    | 续期的Epoch数量，即在原有效期的基础上追加对应的高度                  |  
| new_expiration | 3212223                   | 否    | 不管之前的有效期是多少，统一续期到指定的高度，如果设置了这个值，extension将被忽略 |  
| dry_run        | false                     | 否    | 是否是测试，如果是测试，不会真正执行续期操作，只会做一些检查                |  

#### 请求示例
```json  
{  
 "miner": "f01234", 
 "from": "2024-05-12T07:34:47+00:00", 
 "to": "2024-05-13T07:34:47+00:00", 
 "extension": 20160, 
 "new_expiration": 3212223, 
 "dry_run": false
 }  
```  

#### 返回参数
| 参数名            | 值                                                     | 说明                                        |  
|----------------|-------------------------------------------------------|-------------------------------------------|  
| id             | 11                                                    | 请求ID                                      |  
| extension      | 21000                                                 | 续期追加的高度，创建时指定的参数                          |  
| from           | "2024-05-12T07:34:47Z"                                | 筛选过期sector的开始高度，创建时指定的参数                  |  
| to             | "2024-05-13T07:34:47Z"                                | 筛选过期sector的结束高度，创建时指定的参数                  |  
| new_expiration | null                                                  | 新的过期时间，创建时指定的参数                           |  
| messages       | null                                                  | 续期上链的消息，array                             |  
| miner          | "t017387"                                             | 矿工                                        |  
| status         | "failed"                                              | 状态，`created`,`pending`,`failed`,`success` |  
| took           | 32524714461                                           | 续期执行耗时                                    |  
| confirmed_at   | null                                                  | 消息上链的确认时间                                 |  
| dry_run        | true                                                  | 是否为测试运行                                   |  
| dry_run_result | ""                                                    | 测试运行结果                                    |  
| error          | "failed to get active sector set: RPCConnectionError" | 错误信息                                      |  
| created_at     | "2024-05-11T13:39:40.74831759+08:00"                  | 创建时间                                      |  
| updated_at     | "2024-05-11T13:40:16.237069667+08:00"                 | 更新时间                                      |  

#### 返回示例
```json  
{  
 "data": { 
	 "confirmed_at": null, 
	 "created_at": "2024-05-11T13:39:40.74831759+08:00", 
	 "dry_run": true, 
	 "dry_run_result": "", 
	 "error": "failed to get active sector set: RPCConnectionError", 
	 "extension": 21000, 
	 "from": "2024-05-12T07:34:47Z", 
	 "id": 11, 
	 "messages": null, 
	 "miner": "t017387", 
	 "new_expiration": null, 
	 "status": "created", 
	 "to": "2024-05-13T07:34:47Z", 
	 "took": 32524714461, 
	 "updated_at": "2024-05-11T13:40:16.237069667+08:00"
	 }
}  
``` 

### GET /requests/{:id｝
根据ID查询一个续期请求，返回的结构与创建时一致

#### 返回示例
 ```json
 {  
 "data": { 
	 "confirmed_at": null,
	  "created_at": "2024-05-11T13:39:40.74831759+08:00", 
	  "dry_run": true, 
	  "dry_run_result": "", 
	  "error": "failed to get active sector set: RPCConnectionError", 
	  "extension": 21000, 
	  "from": "2024-05-12T07:34:47Z", 
	  "id": 11, 
	  "messages": null, 
	  "miner": "t017387", 
	  "new_expiration": null, 
	  "status": "failed", 
	  "to": "2024-05-13T07:34:47Z", 
	  "took": 32524714461, 
	  "updated_at": "2024-05-11T13:40:16.237069667+08:00" 
	  }
}
 ```
