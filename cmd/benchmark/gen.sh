protoc \
    --go_out=. \
    --go_opt=Mbenchmark.proto=./pb \
    --go-grpc_out=. \
    --go-grpc_opt=Mbenchmark.proto=./pb \
    ./benchmark.proto