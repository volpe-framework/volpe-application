protoc -I=./volpe-protobuf --go_out=./comms/common --go_opt=paths=source_relative \
--go-grpc_out=./comms/common --go-grpc_opt=paths=source_relative \
common.proto
protoc -I=./volpe-protobuf --go_out=./comms/container --go_opt=paths=source_relative \
--go-grpc_out=./comms/container --go-grpc_opt=paths=source_relative \
volpe_container.proto
protoc -I=./volpe-protobuf --go_out=./comms/volpe --go_opt=paths=source_relative \
--go-grpc_out=./comms/volpe --go-grpc_opt=paths=source_relative \
volpe.proto
