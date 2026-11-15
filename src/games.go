package main

import (
	"fmt"
	"net/netip"

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
	StdinOpen    bool
	Mounts       []mount.Mount
}

func constructMinecraftServerData(name string, port int, memory string, rcon bool) (ServerData, error) {
	tcpPort, err := network.ParsePort("25565/tcp")
	if err != nil {
		return ServerData{}, fmt.Errorf("parsing Minecraft port: %w", err)
	}

	rconPort, err := getMinecraftRconPort()
	if err != nil {
		return ServerData{}, fmt.Errorf("parsing rcon port: %w", err)
	}

	exposedPorts := network.PortSet{tcpPort: struct{}{}}
	publishedPorts := network.PortMap{
		tcpPort: []network.PortBinding{{HostPort: fmt.Sprintf("%d", port)}},
	}
	if rcon {
		exposedPorts[rconPort] = struct{}{}
		publishedPorts[rconPort] = []network.PortBinding{{HostIP: netip.MustParseAddr("127.0.0.1")}}
	}

	return ServerData{
		Name:         name,
		Image:        "gst-minecraft",
		ExposedPorts: exposedPorts,
		PortBindings: publishedPorts,
		Env:          []string{"EULA=TRUE", "MEMORY=" + memory},
		Labels:       map[string]string{"gst-managed": "true"},
		Tty:          true,
		StdinOpen:    true,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: "gst-" + name + "-data", //each server will have its own volume for data persistence
				Target: "/data",
				VolumeOptions: &mount.VolumeOptions{
					Labels: map[string]string{"gst-managed": "true"},
				},
			},
		},
	}, nil
}

func getMinecraftRconPort() (network.Port, error) {
	rconPort, err := network.ParsePort("25575/tcp")
	if err != nil {
		return network.Port{}, fmt.Errorf("failed to parse rcon port: %w", err)
	}
	return rconPort, nil
}
