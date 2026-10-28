module github.com/Pleasurecruise/eyeful

go 1.27.1

require (
	github.com/Pleasurecruise/eyeful/local v0.0.0
	github.com/Pleasurecruise/eyeful/workflow v0.0.0
	github.com/alecthomas/kong v1.16.1
	github.com/sourcegraph/go-diff v0.9.0
)

require (
	github.com/MoeForge/moejs v0.1.0-alpha.11 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.4 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/mattn/go-jev v0.0.3 // indirect
	github.com/owenrumney/go-sarif/v3 v3.3.2 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

replace github.com/Pleasurecruise/eyeful/local => ./local

replace github.com/Pleasurecruise/eyeful/workflow => ./workflow
