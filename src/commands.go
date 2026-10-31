package main

import (
	"context"
	"fmt"
	"net"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ServerInfo struct {
	Managed bool
	Running bool
	IPAddr  string
}

const LOGIN = 3
const DEFAULT_TIMEOUT = 60
const COMMAND = 2

func listServers(clir *client.Client) error {
	res, err := clir.ContainerList(context.Background(), client.ContainerListOptions{
		All:     true,
		Filters: client.Filters{}.Add("label", "gst-managed=true"),
	}) // getting list of containers, filtered to only tool created
	if err != nil {
		return fmt.Errorf("-listServers: %w", err)
	}

	for _, ctr := range res.Items {
		fmt.Printf("Container Name:%s, State:%s\n", strings.TrimPrefix(ctr.Names[0], "/"), ctr.State)
	}

	return nil
}
func startServer(clir *client.Client, name string) error {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("-startServer: %w", err)
	}

	if !inf.Managed {
		return fmt.Errorf("-startServer: Server %s is not managed by the tool", name)
	}

	if inf.Running {
		return fmt.Errorf("-startServer:Server %s is already running\n", name)
	}

	_, err = clir.ContainerStart(context.Background(), name, client.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("-startServer: %w", err)
	}

	fmt.Printf("Server %s started\n", name)

	return nil
}
func stopServer(clir *client.Client, name string) error {
	timeout := DEFAULT_TIMEOUT // seconds

	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("-stopServer: %w", err)
	}

	if !inf.Managed {
		return fmt.Errorf("-stopServer:Server %s is not managed by this tool, cannot stop it\n", name)
	}
	if !inf.Running {
		return fmt.Errorf("-stopServer:Server %s is not running\n", name)
	}
	_, err = clir.ContainerStop(context.Background(), name, client.ContainerStopOptions{
		Timeout: &timeout,
	})

	if err != nil {
		return fmt.Errorf("-stopServer: %w", err)
	}
	fmt.Printf("Server %s stopped\n", name)

	return nil
}
func getServerInfo(clir *client.Client, name string) (ServerInfo, error) {
	res, err := clir.ContainerInspect(context.Background(), name, client.ContainerInspectOptions{})
	if err != nil {
		return ServerInfo{}, fmt.Errorf("-getServerInfo: %w", err) //here dont want to terminate the program, returns false and the error
	}

	ipAddr := ""
	settings := res.Container.NetworkSettings

	if settings != nil {
		endpoint, ok := settings.Networks["bridge"]
		if ok && endpoint != nil {
			ipAddr = endpoint.IPAddress.String()
		}
	}

	return ServerInfo{
		Managed: res.Container.Config.Labels["gst-managed"] == "true",
		Running: res.Container.State.Running,
		IPAddr:  ipAddr,
	}, nil
}
func serverCreate(clir *client.Client, name string, port int, memory string, rcon bool, rconPassword string) error {
	//Future switch case which game, for now only minecraft
	serverData, err := constructMinecraftServerData(name, port, memory)

	if err != nil {
		return fmt.Errorf("-serverCreate: %w", err)
	}
	exists, err := imageExists(clir, serverData.Image)

	if err != nil {
		return fmt.Errorf("-serverCreate: %w", err)
	}
	if !exists {
		return fmt.Errorf("-serverCreate:image %s not found, build it first", serverData.Image)
	}

	if rcon {
		serverData.Env = append(serverData.Env, "RCON=true", "RCON_PASSWORD="+rconPassword)
	}

	containerConfig := &container.Config{
		Image:        serverData.Image,
		Env:          serverData.Env,
		Labels:       serverData.Labels,
		Tty:          serverData.Tty,
		OpenStdin:    serverData.Stdin_open,
		ExposedPorts: serverData.ExposedPorts,
	}
	hostConfig := &container.HostConfig{
		PortBindings:  serverData.PortBindings,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		Mounts:        serverData.Mounts,
	}

	res, err := clir.ContainerCreate(context.Background(), client.ContainerCreateOptions{
		Config:     containerConfig,
		HostConfig: hostConfig,
		Name:       name,
	})
	if err != nil {
		return fmt.Errorf("-serverCreate: %w", err)
	}

	fmt.Printf("Server %s created with ID: %s\n", name, res.ID)

	return nil
}
func serverDelete(clir *client.Client, name string, delVolume bool) error {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("-serverDelete: %w", err)
	}
	if !inf.Managed {
		return fmt.Errorf("-serverDelete:Server %s is not managed by this tool, cannot delete it\n", name)
	}
	if inf.Running {
		err = stopServer(clir, name)
		if err != nil {
			return fmt.Errorf("-serverDelete: %w", err)
		}
	}

	_, err = clir.ContainerRemove(context.Background(), name, client.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("-serverDelete: %w", err)
	}
	fmt.Printf("Server %s deleted\n", name)
	if delVolume {
		volumeName := "gst-" + name + "-data"
		_, err = clir.VolumeRemove(context.Background(), volumeName, client.VolumeRemoveOptions{})
		if cerrdefs.IsNotFound(err) {
			fmt.Printf("No associated volume found, nothing to delete\n")
			return nil
		}
		if err != nil {
			return fmt.Errorf("-serverDelete: %w", err)
		}
		fmt.Printf("Associated volume deleted\n")
	}

	return nil
}
func imageExists(cli *client.Client, imageName string) (bool, error) {
	_, err := cli.ImageInspect(context.Background(), imageName)
	if err == nil {
		return true, nil
	}
	if cerrdefs.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("-imageExists: %w", err)
}
func rconCommand(clir *client.Client, name string, rconPassword string, command string) (ServerResponse, error) {
	res, err := getServerInfo(clir, name)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconCommand: %w", err)
	}
	// sanity checks to ensure the container is on the bridge network and has network settings

	tcpAddr, err := net.ResolveTCPAddr("tcp", res.IPAddr+":25575")
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconCommand: %w", err)
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconCommand: %w", err)
	} //finish setting up connetion

	defer conn.Close()

	_, err = rconLogin(conn, rconPassword)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconCommand: %w", err)
	}

	response, err := interact(conn, int32(COMMAND), command)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconCommand: %w", err)
	}

	return response, nil

}
