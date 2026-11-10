package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ServerInfo struct {
	Managed  bool
	Running  bool
	IPAddr   string
	RCON     bool
	RCONPass string
}

const LOGIN = 3
const DEFAULT_TIMEOUT = 60
const COMMAND = 2
const TIMEOUT_FOR_COMMAND = 20
const TIMEOUT_FOR_CONNECTION = 5

func listServers(clir *client.Client) error {
	res, err := clir.ContainerList(context.Background(), client.ContainerListOptions{
		All:     true,
		Filters: client.Filters{}.Add("label", "gst-managed=true"),
	}) // getting list of containers, filtered to only tool created
	if err != nil {
		return fmt.Errorf("listing servers: %w", err)
	}

	for _, ctr := range res.Items {
		fmt.Printf("Container Name:%s, State:%s\n", strings.TrimPrefix(ctr.Names[0], "/"), ctr.State)
	}

	return nil
}
func startServer(clir *client.Client, name string) error {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("starting server %s: %w", name, err)
	}

	if !inf.Managed {
		return fmt.Errorf("server %s is not managed by gst", name)
	}

	if inf.Running {
		return fmt.Errorf("server %s is already running", name)
	}

	_, err = clir.ContainerStart(context.Background(), name, client.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("starting server %s: %w", name, err)
	}

	fmt.Printf("Server %s started\n", name)

	return nil
}
func stopServer(clir *client.Client, name string) error {
	timeout := DEFAULT_TIMEOUT // seconds

	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("stopping server %s: %w", name, err)
	}

	if !inf.Managed {
		return fmt.Errorf("server %s is not managed by gst", name)
	}
	if !inf.Running {
		return fmt.Errorf("server %s is not running", name)
	}
	_, err = clir.ContainerStop(context.Background(), name, client.ContainerStopOptions{
		Timeout: &timeout,
	})

	if err != nil {
		return fmt.Errorf("stopping server %s: %w", name, err)
	}
	fmt.Printf("Server %s stopped\n", name)

	return nil
}
func getServerInfo(clir *client.Client, name string) (ServerInfo, error) {
	res, err := clir.ContainerInspect(context.Background(), name, client.ContainerInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return ServerInfo{}, fmt.Errorf("server %s was not found", name)
		}
		return ServerInfo{}, fmt.Errorf("inspecting container: %w", err)
	}

	ipAddr := ""
	settings := res.Container.NetworkSettings

	if settings != nil {
		endpoint, ok := settings.Networks["bridge"]
		if ok && endpoint != nil {
			ipAddr = endpoint.IPAddress.String()
		}
	}

	rcon := false
	rconPass := ""
	if res.Container.Config == nil || res.Container.State == nil {
		return ServerInfo{}, fmt.Errorf("container %s has bad config or state", name)
	}

	for _, env := range res.Container.Config.Env {
		if strings.HasPrefix(env, "RCON_PASSWORD=") { // looking for the prefixes we are interested in within env.
			rconPass = strings.TrimPrefix(env, "RCON_PASSWORD=") //trims the prefix to get the password.
		}
		if env == "RCON=true" {
			rcon = true
		}
	}

	return ServerInfo{
		Managed:  res.Container.Config.Labels["gst-managed"] == "true",
		Running:  res.Container.State.Running,
		IPAddr:   ipAddr,
		RCON:     rcon,
		RCONPass: rconPass,
	}, nil
}
func serverCreate(clir *client.Client, name string, port int, memory string, rcon bool) error {
	//Future switch case which game, for now only minecraft
	serverData, err := constructMinecraftServerData(name, port, memory)

	if err != nil {
		return fmt.Errorf("creating server %s: %w", name, err)
	}
	exists, err := imageExists(clir, serverData.Image)

	if err != nil {
		return fmt.Errorf("creating server %s: %w", name, err)
	}
	if !exists {
		return fmt.Errorf("image %s not found, build it first", serverData.Image)
	}

	if rcon {
		rconPass := rand.Text()
		serverData.Env = append(serverData.Env, "RCON=true", "RCON_PASSWORD="+rconPass)
	}

	containerConfig := &container.Config{
		Image:        serverData.Image,
		Env:          serverData.Env,
		Labels:       serverData.Labels,
		Tty:          serverData.Tty,
		OpenStdin:    serverData.StdinOpen,
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
		return fmt.Errorf("creating server %s: %w", name, err)
	}

	fmt.Printf("Server %s created with ID: %s\n", name, res.ID)
	fmt.Printf("Starting server %s...\n", name)

	err = startServer(clir, name)
	if err != nil {
		fmt.Printf("Failed to start server %s, cleaning up...\n", name)
		e := serverDelete(clir, name, false) // trying to clean up the server if starting fails
		if e != nil {
			fmt.Printf("Failed to clean up server %s: %v\n", name, e)
		}
		return fmt.Errorf("server %s was created but failed to start (is port %d in use?): %w", name, port, err)
	}

	return nil
}
func serverDelete(clir *client.Client, name string, delVolume bool) error {
	inf, err := getServerInfo(clir, name)
	if err != nil {
		return fmt.Errorf("deleting server %s: %w", name, err)
	}
	if !inf.Managed {
		return fmt.Errorf("server %s is not managed by gst", name)
	}
	if inf.Running {
		err = stopServer(clir, name)
		if err != nil {
			return fmt.Errorf("deleting server %s: %w", name, err)
		}
	}

	_, err = clir.ContainerRemove(context.Background(), name, client.ContainerRemoveOptions{})
	if err != nil {
		return fmt.Errorf("deleting server %s: %w", name, err)
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
			return fmt.Errorf("removing volume %s: %w", volumeName, err)
		}
		fmt.Printf("Associated volume deleted\n")
	}

	return nil
}
func imageExists(clir *client.Client, imageName string) (bool, error) {
	_, err := clir.ImageInspect(context.Background(), imageName)
	if err == nil {
		return true, nil
	}
	if cerrdefs.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("inspecting image %s: %w", imageName, err)
}
func rconCommand(clir *client.Client, name string, command string) (ServerResponse, error) {
	res, err := getServerInfo(clir, name)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("sending command to %s: %w", name, err)
	}

	if !res.Managed {
		return ServerResponse{}, fmt.Errorf("server %s is not managed by gst", name)
	}

	if !res.Running {
		return ServerResponse{}, fmt.Errorf("server %s is not running", name)
	}

	if !res.RCON {
		return ServerResponse{}, fmt.Errorf("RCON is not enabled on server %s, create it with -rcon", name)
	}
	rconPassword := res.RCONPass

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(res.IPAddr, "25575"), TIMEOUT_FOR_CONNECTION*time.Second)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("connecting to RCON on %s: %w", name, err)
	} //finish setting up connetion

	defer conn.Close()

	conn.SetDeadline(time.Now().Add(TIMEOUT_FOR_COMMAND * time.Second)) //sets a deadline for all read and write operations on the connection

	_, err = rconLogin(conn, rconPassword)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("sending command to %s: %w", name, err)
	}

	response, err := interact(conn, int32(COMMAND), command)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("sending command to %s: %w", name, err)
	}

	return response, nil

}
