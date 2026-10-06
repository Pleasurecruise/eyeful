module github.com/Pleasurecruise/eyeful

go 1.27.1

require (
	github.com/Pleasurecruise/eyeful/local v0.0.0
	github.com/Pleasurecruise/eyeful/workflow v0.0.0
	github.com/alecthomas/kong v1.16.1
	github.com/golang-migrate/migrate/v4 v4.20.1
	github.com/google/go-github/v92 v92.0.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/labstack/echo/v5 v5.4.0
	github.com/swaggo/swag/v2 v2.0.0-rc6
	go.uber.org/fx v1.24.0
	golang.org/x/oauth2 v0.37.0
)

require (
	cel.dev/expr v0.25.2 // indirect
	filippo.io/edwards25519 v1.1.1 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.2 // indirect
	github.com/coder/acp-go-sdk v0.13.5 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/cubicdaiya/gonp v1.0.4 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fatih/structtag v1.2.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/spec v0.20.9 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-sql-driver/mysql v1.9.3 // indirect
	github.com/google/cel-go v0.28.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/pgerrcode v0.0.0-20220416144525-469b46aa5efa // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/ncruces/go-sqlite3 v0.32.0 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/owenrumney/go-sarif/v3 v3.3.1 // indirect
	github.com/pganalyze/pg_query_go/v6 v6.2.2 // indirect
	github.com/pingcap/errors v0.11.5-0.20250523034308-74f78ae071ee // indirect
	github.com/pingcap/failpoint v0.0.0-20240528011301-b51a646c7c86 // indirect
	github.com/pingcap/log v1.1.0 // indirect
	github.com/pingcap/tidb/pkg/parser v0.0.0-20260418072757-ce92298d1124 // indirect
	github.com/riza-io/grpc-go v0.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sourcegraph/go-diff v0.9.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/sqlc-dev/doubleclick v1.0.0 // indirect
	github.com/sqlc-dev/sqlc v1.31.1 // indirect
	github.com/sv-tools/openapi v0.4.0 // indirect
	github.com/tetratelabs/wazero v1.11.0 // indirect
	github.com/urfave/cli/v2 v2.27.5 // indirect
	github.com/wasilibs/go-pgquery v0.0.0-20250409022910-10ac41983c07 // indirect
	github.com/wasilibs/wazero-helpers v0.0.0-20240620070341-3dff1577cd52 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/grpc v1.82.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

tool (
	github.com/sqlc-dev/sqlc/cmd/sqlc
	github.com/swaggo/swag/v2/cmd/swag
)

replace github.com/Pleasurecruise/eyeful/local => ./local

replace github.com/Pleasurecruise/eyeful/workflow => ./workflow
