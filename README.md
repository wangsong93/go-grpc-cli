# go-grpc-cli

## about
grpc cli written in go.

commands:
```
ls - list services
lsm - list services methods
find_method - try to find specified method, returns 0 on success 1 otherwise
```

for more info add `-help` argument 

## installing
```
go get github.com/minaevmike/go-grpc-cli
```
## server reflection
To enable server reflection see [this](https://github.com/grpc/grpc-go/blob/master/Documentation/server-reflection-tutorial.md) tutorial

## usage
```
go build
./go-grpc-cli -address localhost:9999 -cmd ls
./go-grpc-cli -address localhost:9999 -cmd lsm
```