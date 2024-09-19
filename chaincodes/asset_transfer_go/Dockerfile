FROM golang:1.16

WORKDIR /app
COPY  assetTransfer.go go.mod go.sum ./
COPY chaincode/ ./chaincode/
RUN ls -la ./ \
    && GO111MODULE=on GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -o asset_transfer.bin assetTransfer.go
    