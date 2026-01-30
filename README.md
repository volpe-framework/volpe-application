# VolPE
_Volunteer Computing Platform for Evolutionary Algorithms_

This repository houses the primary components of VolPE, i.e. the master and the worker applications.

## Master Application

The following are instructions to get the master application up and running:

1. Install `podman` and the Go toolchain
2. Setup podman appropriately
3. Download this repository
4. Export the environment variable `CONTAINER_HOST`. Value can be determined by running `podman info -f json | jq .host.remoteSocket.path`. The output from the command must be modified as follows.If the output is `"/run/user/1000/podman/podman.sock"`, the following command must be used to export the env. var.
```
export CONTAINER_HOST=unix:///run/user/1000/podman/podman.sock
```
5. `cd` into the `master/` folder and run `go run .`
6. Install any missing dependencies and repeat 4, till the application is running


## Worker Application

1. Install `podman` and the Go toolchain
1. Install `podman` and the Go toolchain
2. Setup podman appropriately
3. Download this repository
4. Export the environment variable `CONTAINER_HOST`. Value can be determined by running `podman info -f json | jq .host.remoteSocket.path`. The output from the command must be modified as follows.If the output is `"/run/user/1000/podman/podman.sock"`, the following command must be used to export the env. var.
```
export CONTAINER_HOST=/run/user/1000/podman/podman.sock
```
5. Set the `VOLPE_MASTER` environment variable to the IP address where the master is accessible, with port 8000 by default. For example, if the master is accessible at IP address 192.168.0.2:
```
export VOLPE_MASTER=192.168.0.2:8080
```
6. `cd` into the `worker/` folder and run `go run .`
7. Install any missing dependencies and repeat 6, till the application is running
