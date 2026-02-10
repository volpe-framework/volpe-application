// Package container_mgr provides Podman container management
package container_mgr

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/containers"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/specgen"
)

// crates podman connection
func NewPodmanConnection() (context.Context, error) {
	uid := os.Getuid()
	containerHost, ok := os.LookupEnv("CONTAINER_HOST")
	if !ok {
		containerHost = fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", uid)
		log.Warn().Msgf("Using default Podman socket expected at %s. Please set the CONTAINER_HOST env var.", containerHost)
	}
	conn, err := bindings.NewConnection(context.Background(), containerHost)
	return conn, err
}

// loads image and runs as new podman container and returns host port
func runImage(imagePath string, containerName string, containerPort uint16) (uint16, error) {
	conn, err := NewPodmanConnection()
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("error creating podman connection: %s", err.Error()))
		return 0, err
	}

	dir, _ := os.Getwd()
	log.Warn().Msgf("running from %s", dir)

	f, err := os.Open(imagePath)
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("error opening img file: %s", err.Error()))
		return 0, err
	}

	imgLoadReport, err := images.Load(conn, f) //, &images.LoadOptions{Reference: &imageName})
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("error importing image from %s: %s", imagePath, err.Error()))
		return 0, err
	}
	log.Info().Msg(fmt.Sprintf("loaded image %s from file %s\n", imgLoadReport.Names[0], imagePath))

	imageName := imgLoadReport.Names[0]

	ignore := true
	_ = containers.Stop(conn, containerName, &containers.StopOptions{Ignore: &ignore})
	_, _ = containers.Remove(conn, containerName, &containers.RemoveOptions{Ignore: &ignore})

	sg := specgen.NewSpecGenerator(imageName, false)
	remove := true
	terminal := true
	sg.Remove = &remove
	sg.Name = containerName
	pm := types.PortMapping{ContainerPort: containerPort}
	sg.PortMappings = []types.PortMapping{pm}
	sg.Terminal = &terminal
	cr, err := containers.CreateWithSpec(conn, sg, nil)
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("could not create container with image %s: %s", imageName, err.Error()))
		return 0, err
	}
	err = containers.Start(conn, cr.ID, nil)
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("could not start container %s: %s", cr.ID, err.Error()))
		return 0, err
	}

	// outFile, _ := os.Create(containerName + "_logs")
	// errFile, _ := os.Create(containerName + "_err")
	go func() {
		err := containers.Attach(conn, cr.ID, nil, os.Stdout, os.Stderr, nil, nil)
		if err != nil {
			log.Error().Caller().Msgf("could not attach container %s: %s", cr.ID, err.Error())
		}
	}()

	inspectContainerData, _ := containers.Inspect(conn, containerName, nil)
	hostPort, _ := strconv.Atoi(inspectContainerData.HostConfig.PortBindings[fmt.Sprintf("%d/tcp", containerPort)][0].HostPort)
	log.Info().Msg("successfully running container from " + imagePath)
	return uint16(hostPort), nil
}

// stops a running podman container
func stopContainer(containerName string) {
	conn, err := NewPodmanConnection()
	if err != nil {
		log.Error().Caller().Msg(fmt.Sprintf("error creating podman connection: %s", err.Error()))
	}
	_ = containers.Stop(conn, containerName, nil)
}

