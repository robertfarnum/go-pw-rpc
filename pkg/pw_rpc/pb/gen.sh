protoc \
    --go_out=. \
    --go_opt=Mpacket.proto=../pb \
    ./packet.proto
protoc \
    --go_out=. \
    --go_opt=Mstatus.proto=../pb \
    ./status.proto