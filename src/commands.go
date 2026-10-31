package main

import (
	"context"
	"fmt"
	"log"
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

func listServers(clir *client.Client) {
	res, err := clir.ContainerList(context.Background(), client.ContainerListOptions{
		All:     true,
		Filters: client.Filters{}.Add("label", "gst-managed=true"),
	}) // getting list of containers, filtered to only tool created
	if err != nil {
		log.Fatal(err)
	}

	for _, ctr := range res.Items {
		fmt.Printf("Container Name:%s, State:%s\n", strings.TrimPrefix(ctr.Names[0], "/"), ctr.State)
	}
}
func startServer(clir *client.Client, name string) {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		log.Fatal(err)
	}

	if !inf.Managed {
		fmt.Printf("Server %s is not managed by this tool, cannot start it\n", name)
		return
	}

	if inf.Running {
		fmt.Printf("Server %s is already running\n", name)
		return
	}

	_, err = clir.ContainerStart(context.Background(), name, client.ContainerStartOptions{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Server %s started\n", name)
}
func stopServer(clir *client.Client, name string) {
	timeout := DEFAULT_TIMEOUT // seconds

	inf, err := getServerInfo(clir, name)
	if err != nil {
		log.Fatal(err)
	}

	if !inf.Managed {
		fmt.Printf("Server %s is not managed by this tool, cannot stop it\n", name)
		return
	}
	if !inf.Running {
		fmt.Printf("Server %s is not running\n", name)
		return
	}

	_, err = clir.ContainerStop(context.Background(), name, client.ContainerStopOptions{
		Timeout: &timeout,
	})

	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server %s stopped\n", name)
}
func getServerInfo(clir *client.Client, name string) (ServerInfo, error) {
	res, err := clir.ContainerInspect(context.Background(), name, client.ContainerInspectOptions{})
	if err != nil {
		return ServerInfo{}, err //here dont want to terminate the program, returns false and the error
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
func serverCreate(clir *client.Client, name string, port int, memory string, rcon bool, rconPassword string) {
	//Future switch case which game, for now only minecraft
	serverData, err := constructMinecraftServerData(name, port, memory)

	if err != nil {
		log.Fatal(err)
	}
	exists, err := imageExists(clir, serverData.Image)
	if err != nil {
		log.Fatal(err)
	}
	if !exists {
		fmt.Printf("image %s not found, build it first", serverData.Image)
		return
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
		log.Fatal(err)
	}
	fmt.Printf("Server %s created with ID: %s\n", name, res.ID)
}
func serverDelete(clir *client.Client, name string, delVolume bool) {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		log.Fatal(err)
	}
	if !inf.Managed {
		fmt.Printf("Server %s is not managed by this tool, cannot delete it\n", name)
		return
	}
	if inf.Running {
		stopServer(clir, name)
	}

	_, err = clir.ContainerRemove(context.Background(), name, client.ContainerRemoveOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server %s deleted\n", name)
	if delVolume {
		volumeName := "gst-" + name + "-data"
		_, err = clir.VolumeRemove(context.Background(), volumeName, client.VolumeRemoveOptions{})
		if cerrdefs.IsNotFound(err) {
			fmt.Printf("No associated volume found, nothing to delete\n")
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Associated volume deleted\n")
	}
}
func imageExists(cli *client.Client, imageName string) (bool, error) {
	_, err := cli.ImageInspect(context.Background(), imageName)
	if err == nil {
		return true, nil
	}
	if cerrdefs.IsNotFound(err) {
		return false, nil
	}
	return false, err
}
func rconCommand(clir *client.Client, name string, rconPassword string, command string) (ServerResponse, error) {
	res, err := getServerInfo(clir, name)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("getting server info failed: %w", err)
	}
	// sanity checks to ensure the container is on the bridge network and has network settings

	tcpAddr, err := net.ResolveTCPAddr("tcp", res.IPAddr+":25575")
	if err != nil {
		return ServerResponse{}, fmt.Errorf("ResolveTCPAddr failed:%v", err)
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("Dial failed:%v", err)
	} //finish setting up connetion

	defer conn.Close()

	_, err = rconLogin(conn, rconPassword)
	if err != nil {
		return ServerResponse{}, err
	}

	response, err := interact(conn, int32(COMMAND), command)
	if err != nil {
		return ServerResponse{}, err
	}

	return response, nil

}
