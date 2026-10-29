package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/moby/moby/client"
)

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

		if createCmd.NArg() != 1 { // only Name remain as a non flag
			fmt.Println("Usage: gst create [-port N] [-memory SIZE] <serverName>")
			os.Exit(1)
		}
		if !validMemory(*memory) {
			fmt.Printf("Invalid memory value %q, expected 1G to 32G\n", *memory)
			os.Exit(1)
		}
		name := createCmd.Arg(0)
		serverCreate(clir, name, *port, *memory)
	case "delete":
		deleteCmnd:= flag.NewFlagSet("delete", flag.ExitOnError)
		delVolume := deleteCmnd.Bool("delvolume", false, "delete associated volume")
		deleteCmnd.Parse(os.Args[2:])

		if deleteCmnd.NArg() != 1 { // only Name remain as a non flag
			fmt.Println("Usage: gst delete [-delvolume] <serverName>")
			os.Exit(1)
		}
		name := deleteCmnd.Arg(0)
		serverDelete(clir, name, *delVolume)
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

const helpText = `Usage: gst <command> [arguments]

Commands:
  create [options] <serverName>
                       Create a new Minecraft server container with the given name
      -port N          Host port to expose (default 25565)
      -memory SIZE     Server memory, 1G to 32G (default 4G)
  start <serverName>   Start the game server container with the given name
  stop <serverName>    Stop the game server gracefully (waits up to 60 seconds)
  delete [options] <serverName>
                       Delete the server container, stopping it first if running
      -delvolume       Also delete the server's data volume (world data is lost)
  list                List all servers managed by this tool and their state
  help                 Show this help message

Examples:
  gst create survival
  gst create -port 25570 -memory 8G creative
  gst list
  gst start survival
  gst stop survival
  gst delete -delvolume survival`

func printHelp() {
	fmt.Println(helpText)
}
