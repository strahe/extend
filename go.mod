module github.com/strahe/extend

go 1.23.7

toolchain go1.24.2

require (
	github.com/filecoin-project/go-address v1.2.0
	github.com/filecoin-project/go-bitfield v0.2.4
	github.com/filecoin-project/go-jsonrpc v0.7.0
	github.com/filecoin-project/go-state-types v0.16.0
	github.com/filecoin-project/lotus v1.32.2
	github.com/go-playground/validator/v10 v10.20.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/gorilla/mux v1.8.1
	github.com/ipfs/go-cid v0.5.0
	github.com/ipfs/go-ipld-cbor v0.2.0
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/samber/lo v1.39.0
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli/v2 v2.27.5
	github.com/xo/dburl v0.23.2
	go.uber.org/zap v1.27.0
	gorm.io/driver/mysql v1.5.7
	gorm.io/driver/postgres v1.5.9
	gorm.io/driver/sqlite v1.5.5
	gorm.io/gorm v1.25.10
)

replace github.com/filecoin-project/filecoin-ffi => ./extern/filecoin-ffi

replace google.golang.org/genproto => google.golang.org/genproto/googleapis/api v0.0.0-20250212204824-5a70512c5d8b
