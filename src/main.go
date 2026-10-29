package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

type ServerInfo struct {
	Managed bool
	Running bool
}

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

func main() {

	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}
	clir, err := client.New(client.FromEnv) // connecting to docker using standart settings
	if err != nil {
		log.Fatal(err)
	}
	defer clir.Close() // runs when main function ends

	switch os.Args[1] {
	case "list":
		listServers(clir)
	case "start", "stop":
		if len(os.Args) < 3 {
			fmt.Printf("Missing server name. Usage: gst %s <serverName>\n", os.Args[1])
			os.Exit(1)
		}
		if os.Args[1] == "start" {
			startServer(clir, os.Args[2])
		} else {
			stopServer(clir, os.Args[2])
		}
	case "create":
		if len(os.Args) < 3 {
			fmt.Printf("Missing server name. Usage: gst create <serverName>\n")
			os.Exit(1)
		}
		serverCreate(clir, os.Args[2])
	case "help":
		printHelp()
	default:
		fmt.Println("Unknown command, refer to gst help for usage")
	}
}

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
	timeout := 60 // seconds

	inf, err := getServerInfo(clir, name)
	if err != nil {
		log.Fatal(err)
	}

	if !inf.Managed {
		fmt.Printf("Server %s is not managed by this tool, cannot stop it\n", name)
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

	return ServerInfo{
		Managed: res.Container.Config.Labels["gst-managed"] == "true",
		Running: res.Container.State.Running,
	}, nil
}
func constructMinecraftServerData(name string) (ServerData, error) {
	tcpPort, err := network.ParsePort("25565/tcp")
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
			tcpPort: []network.PortBinding{{HostPort: "25565"}},
		},
		Env:        []string{"EULA=TRUE", "MEMORY=4G"},
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
func serverCreate(clir *client.Client, name string) {
	//Future switch case which game, for now only minecraft
	serverData, err := constructMinecraftServerData(name)
	if err != nil {
		log.Fatal(err)
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

const helpText = `Usage: gst <command> [arguments]

Commands:
  start <serverName>   Start the game server container with the given name
  stop <serverName>    Stop the game server gracefully (waits up to 60 seconds)
  list                 List all servers managed by this tool and their state
  help                 Show this help message

Examples:
  gst list
  gst start survival
  gst stop survival`

func printHelp() {
	fmt.Println(helpText)
}
