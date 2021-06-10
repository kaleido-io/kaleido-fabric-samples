module github.com/kaleido-io/kaleido-fabric-go

go 1.15

require (
	github.com/hyperledger/fabric-sdk-go v1.0.1-0.20210201220314-86344dc25e5d
	github.com/kaleido-io/kaleido-sdk-go v0.0.0-20210419110505-7c6d2c9f5b46
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/kaleido-io/kaleido-sdk-go => ../../kaleido-sdk-go
