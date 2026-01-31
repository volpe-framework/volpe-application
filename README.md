# VolPE
_Volunteer Computing Platform for Evolutionary Algorithms_

This repository houses the primary components of VolPE, i.e. the master and the worker applications.

## Dependencies
1. Install `podman` and the Go toolchain
2. Setup podman appropriately
3. Download this repository
4. Export the environment variable `CONTAINER_HOST`. Value can be determined by running `podman info -f json | jq .host.remoteSocket.path`. The output from the command must be modified as follows.If the output is `"/run/user/1000/podman/podman.sock"`, the following command must be used to export the env. var.
```
export CONTAINER_HOST=unix:///xyz.sock
```

## Master Application

The following are instructions to get the master application up and running in different platforms:

### Linux / MacOS / Windows:
1. `cd` into the `master/` folder and run `go run .`

## Worker Application
The following are instructions to get the worker application up and running in different platforms:

### Linux / MacOS / Windows:
1. Set the `VOLPE_MASTER` environment variable to the IP address where the master is accessible, with port 8000 by default. For example, if the master is accessible at IP address 192.168.0.2:
```
export VOLPE_MASTER=192.168.0.2:8080
```
2. `cd` into the `worker/` folder and run `go run .`

### Steps to Fix (WSL)

To change the firewall configs in WSL do

`
mkdir -p ~/.config/containers
nano ~/.config/containers/containers.conf
`

And add this line

``
[network]
firewall_driver = "iptables"
`` 

NOTE
Shutdown the WSL. And reboot. Only then the changes reflect

`
wsl --shutdown          # Shutdown wsl
wsl -d <distro-name>    # Reboot
`

## Building and Deploying a Problem
1. Clone repo `https://github.com/aadit-n3rdy/volpe-py.git`
2. cd into `volpe-py/`
3. Run ``bash build-image.sh``
4. Ensure `.tar` file present

## Running in the Website
1. Open browser and go to `localhost:8000/static`
2. Give the problem a name and choose .tar file built from `volke-py/` and enter the memory count and number of target instances
3. Enter the problem name and click `START PROBLEM`
4. Enter the problem name and and click `CONNECT STREAM`
