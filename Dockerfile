# syntax=docker/dockerfile:1

FROM docker.io/golang:1.24 as build-stage

WORKDIR /app

RUN apt-get update -y && apt-get upgrade -y
RUN apt-get install -y libbtrfs-dev libgpgme-dev

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download


# Copy the source code. Note the slash at the end, as explained in
# https://docs.docker.com/reference/dockerfile/#copy
COPY . .

# Build
RUN mkdir /volpe
RUN cd ./worker && go build -o /volpe/ . 
RUN cd ./master && go build -o /volpe/ . 

FROM quay.io/podman/stable

COPY --from=build-stage /volpe /home/podman/
USER podman
WORKDIR /home/podman/

# podman run --security-opt label=disable --user podman --device /dev/fuse quay.io/podman/stable podman run alpine echo hello
