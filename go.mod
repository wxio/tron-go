module github.com/wxio/tron-go

go 1.12

//replace golang.org/x/tools => github.com/wxio/tools v0.0.1
replace golang.org/x/tools => /home/garym/devel/wxio/golang_tools

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.1
	github.com/golangq/q v1.0.7
	github.com/jpillora/opts v1.0.5
	github.com/wxio/goantlr v0.0.0-20190624051626-116617327c90
	golang.org/x/tools v0.0.0-00010101000000-000000000000
)
