package main

import (
	"fmt"

	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
)

type ServerData struct {
	Name         string
	Image        string
	ExposedPorts network.PortSet
	PortBindings network.PortMap
	Env          []string
	Labels       map[string]string
	Tty          bool
	Stdin_open   bool
	Mounts       []mount.Mount
}

func constructMinecraftServerData(name string, port int, memory string) (ServerData, error) {
	tcpPort, err := network.ParsePort(fmt.Sprintf("%d/tcp", port))
	if err != nil {
		return ServerData{}, fmt.Errorf("parsing minecraft port: %w", err)
	}

	return ServerData{
		Name:  name,
		Image: "gst-minecraft",
		ExposedPorts: network.PortSet{
			tcpPort: struct{}{},
		},
		PortBindings: network.PortMap{
			tcpPort: []network.PortBinding{{HostPort: fmt.Sprintf("%d", port)}},
		},
		Env:        []string{"EULA=TRUE", fmt.Sprintf("MEMORY=%s", memory)},
		Labels:     map[string]string{"gst-managed": "true"},
		Tty:        true,
		Stdin_open: true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: "gst-" + name + "-data", //each server will have its own volume for data persistence
				Target: "/data",
			},
		},
	}, nil
}
