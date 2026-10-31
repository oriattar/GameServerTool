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

const SERVER_NAME = 0
const TOOL_COMMAND = 1
const COMMAND_MIN_ARGS = 2
const CMD_MIN_ARGS = 2

func main() {

	if len(os.Args) < COMMAND_MIN_ARGS {
		printHelp()
		os.Exit(1)
	}
	clir, err := client.New(client.FromEnv) // connecting to docker using standart settings
	if err != nil {
		log.Fatal(err)
	}
	defer clir.Close() // runs when main function ends

	args := os.Args[COMMAND_MIN_ARGS:] // arguments after the command name

	switch os.Args[TOOL_COMMAND] {
	case "list":
		listServers(clir)
	case "start":
		handleStart(clir, args)
	case "stop":
		handleStop(clir, args)
	case "create":
		handleCreate(clir, args)
	case "delete":
		handleDelete(clir, args)
	case "cmd":
		handleCmd(clir, args)
	case "help":
		printHelp()
	default:
		printHelp()
	}
}

func handleStart(clir *client.Client, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: gst start <serverName>")
		os.Exit(1)
	}
	startServer(clir, args[SERVER_NAME])
}

func handleStop(clir *client.Client, args []string) {
	if len(args) != 1 {
		fmt.Println("Usage: gst stop <serverName>")
		os.Exit(1)
	}
	stopServer(clir, args[SERVER_NAME])
}

func handleCreate(clir *client.Client, args []string) {
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	port := createCmd.Int("port", 25565, "host port")
	memory := createCmd.String("memory", "4G", "server memory")
	eula := createCmd.Bool("eula", false, "accept the Minecraft EULA (required)")
	rcon := createCmd.Bool("rcon", false, "enable RCON (remote console) access")
	rconPassword := createCmd.String("rconPass", "", "RCON password (required if RCON is enabled)")

	createCmd.Parse(args)

	if createCmd.NArg() != 1 { // only Name remain as a non flag
		fmt.Println("Usage: gst create -eula [-port N] [-memory SIZE] [-rcon -rconPass PASS] <serverName>")
		os.Exit(1)
	}
	if !*eula {
		fmt.Println("EULA not accepted. Add -eula to accept it, e.g. gst create -eula <serverName>")
		os.Exit(1)
	}
	if !validMemory(*memory) {
		fmt.Printf("Invalid memory value %q, expected 1G to 32G\n", *memory)
		os.Exit(1)
	}
	if *rcon && *rconPassword == "" {
		fmt.Println("RCON password is required with -rcon. Add -rconPass <password>")
		os.Exit(1)
	}
	name := createCmd.Arg(SERVER_NAME)
	serverCreate(clir, name, *port, *memory, *rcon, *rconPassword)
}

func handleDelete(clir *client.Client, args []string) {
	deleteCmnd := flag.NewFlagSet("delete", flag.ExitOnError)
	delVolume := deleteCmnd.Bool("delvolume", false, "delete associated volume")
	deleteCmnd.Parse(args)

	if deleteCmnd.NArg() != 1 { // only Name remain as a non flag
		fmt.Println("Usage: gst delete [-delvolume] <serverName>")
		os.Exit(1)
	}
	name := deleteCmnd.Arg(SERVER_NAME)
	serverDelete(clir, name, *delVolume)
}

func handleCmd(clir *client.Client, args []string) {
	cmdCmnd := flag.NewFlagSet("cmd", flag.ExitOnError)
	rconPass := cmdCmnd.String("rconPass", "", "RCON password of the server (required)")
	cmdCmnd.Parse(args)

	if cmdCmnd.NArg() < CMD_MIN_ARGS { // at least one command word and Name remain as non flags
		fmt.Println("Usage: gst cmd -rconPass <password> <command> <serverName>")
		os.Exit(1)
	}
	if *rconPass == "" {
		fmt.Println("RCON password is required. Add -rconPass <password>")
		os.Exit(1)
	}

	command, name := splitCmdArgs(cmdCmnd.Args())
	response, err := rconCommand(clir, name, *rconPass, command)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Response)
}

// splitCmdArgs treats the last argument as the server name and joins the rest into the command
func splitCmdArgs(args []string) (string, string) {
	last := len(args) - 1
	return strings.Join(args[:last], " "), args[last]
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
  create -eula [options] <serverName>
                       Create a new Minecraft server container with the given name
      -eula            Accept the Minecraft EULA (required)
      -port N          Host port to expose (default 25565)
      -memory SIZE     Server memory, 1G to 32G (default 4G)
      -rcon            Enable RCON so the server accepts gst cmd
      -rconPass PASS   RCON password (required with -rcon)
  start <serverName>   Start the game server container with the given name
  stop <serverName>    Stop the game server gracefully (waits up to 60 seconds)
  delete [options] <serverName>
                       Delete the server container, stopping it first if running
      -delvolume       Also delete the server's data volume (world data is lost)
  cmd -rconPass <password> <command> <serverName>
                       Send a console command to a running server over RCON
                       The last argument is the server name, the rest is the command
      -rconPass PASS   RCON password the server was created with (required)
  list               List all servers managed by this tool and their state
  help                 Show this help message

Examples:
  gst create -eula survival
  gst create -eula -port 25570 -memory 8G creative
  gst create -eula -rcon -rconPass secret survival
  gst list
  gst start survival
  gst stop survival
  gst delete -delvolume survival
  gst cmd -rconPass secret say hello survival`

func printHelp() {
	fmt.Println(helpText)
}
