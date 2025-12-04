
.PHONY: all common container_comms volpe_comms

all: container_comms volpe_comms

common: common.proto
	protoc -I=. --go_out=../common --go_opt=paths=source_relative \
	--go-grpc_out=../common --go-grpc_opt=paths=source_relative \
	common.proto

	python3 -m grpc_tools.protoc -I. --python_out=../pybindings --pyi_out=../pybindings \
	--grpc_python_out=../pybindings common.proto

container_comms: volpe_container.proto common
	protoc -I=. --go_out=../container --go_opt=paths=source_relative \
	--go-grpc_out=../container --go-grpc_opt=paths=source_relative \
	volpe_container.proto

	python3 -m grpc_tools.protoc -I. --python_out=../pybindings --pyi_out=../pybindings \
	--grpc_python_out=../pybindings volpe_container.proto

volpe_comms: volpe.proto common
	protoc -I=. --go_out=../volpe --go_opt=paths=source_relative \
	--go-grpc_out=../volpe --go-grpc_opt=paths=source_relative \
	volpe.proto

