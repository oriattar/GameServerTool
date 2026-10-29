package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

type ServerInfo struct {
	Managed bool
	Running bool
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
		createCmd := flag.NewFlagSet("create", flag.ExitOnError)
		port := createCmd.Int("port", 25565, "host port")
		memory := createCmd.String("memory", "4G", "server memory")
		createCmd.Parse(os.Args[2:])

		if createCmd.NArg() < 1 {
			fmt.Println("Usage: gst create -port [N] -memory [[NumOfGigabytes]G] <serverName>")
			os.Exit(1)
		}
		if !validMemory(*memory) {
			fmt.Printf("Invalid memory value %q, expected 1G to 32G\n", *memory)
			os.Exit(1)
		}
		name := createCmd.Arg(0)
		serverCreate(clir, name, *port, *memory)
	case "help":
		printHelp()
	default:
		printHelp()
	}
}

// validMemory checks the value is a positive number of gigabytes, e.g. "4G"
func validMemory(memory string) bool {
	num, found := strings.CutSuffix(memory, "G")
	if !found {
		return false
	}
	n, err := strconv.Atoi(num)
	return err == nil && n > 0 && n <= 32
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

	return ServerInfo{
		Managed: res.Container.Config.Labels["gst-managed"] == "true",
		Running: res.Container.State.Running,
	}, nil
}
func serverCreate(clir *client.Client, name string, port int, memory string) {
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

const helpText = `Usage: gst <command> [arguments]

Commands:
  create [options] <serverName>
                       Create a new Minecraft server container with the given name
      -port N          Host port to expose (default 25565)
      -memory SIZE     Server memory, 1G to 32G (default 4G)
  start <serverName>   Start the game server container with the given name
  stop <serverName>    Stop the game server gracefully (waits up to 60 seconds)
  list                 List all servers managed by this tool and their state
  help                 Show this help message

Examples:
  gst create survival
  gst create -port 25570 -memory 8G creative
  gst list
  gst start survival
  gst stop survival`

func printHelp() {
	fmt.Println(helpText)
}
