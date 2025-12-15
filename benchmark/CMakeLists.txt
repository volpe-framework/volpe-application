cmake_minimum_required(VERSION 3.10)

project(my_project VERSION 1.0
  LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 14)
set(CMAKE_CXX_STANDARD_REQUIRED True)
set(CMAKE_CXX_EXTENSIONS OFF) # Disallow compiler-specific extensions

find_package(Protobuf REQUIRED)
find_package(gRPC REQUIRED)

add_executable(volpe_problem
  main.cpp
  volpe_container.grpc.pb.cc
  volpe_container.pb.cc
  common.grpc.pb.cc
  common.pb.cc
  volpe_problem.cpp
)

target_link_libraries(volpe_problem PRIVATE
    protobuf::libprotobuf
    gRPC::grpc++
    gRPC::grpc
    gRPC::gpr
    Threads::Threads
)

target_include_directories(volpe_problem PRIVATE
    ./
    ./SEvoBench/include/
)
