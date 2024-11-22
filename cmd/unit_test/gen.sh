protoc \
    --go_out=. \
    --go_opt=Munit_test.proto=./pb \
    --go-grpc_out=. \
    --go-grpc_opt=Munit_test.proto=./pb \
    ./unit_test.proto