# GameServerTool

GameServerTool (`gst`) is a command-line tool, written in Go, that creates and manages game servers running in Docker containers. Instead of writing `docker run` commands by hand, you create, start, stop, delete and talk to your servers with short commands like `gst create -eula survival`.

Each server runs in its own container, and its world data lives in its own Docker volume, so it survives restarts and container upgrades.

## Why I built it

This is a learning project. My goals were to:

- **Learn Docker:** working with Docker, images, volumes and Compose, and driving them from code through the Docker Go SDK.
- **Learn Go as a programming language:** a real CLI with flags, error handling, tests, and a binary network protocol (RCON).
- **Learn to work with AI tools while writing a project:** using Claude Code and GitHub Copilot Pro as part of the development process.

## How it works

Currently only **Minecraft (Java Edition)** is supported. The code is structured so more games can be added later.

- **Image:** the `Dockerfile` builds an image called `gst-minecraft` from the Eclipse Temurin Java runtime and the official Minecraft server jar.
- **Entrypoint:** `entrypoint.sh` runs when the container starts. It checks that the EULA was accepted and the memory value is valid, writes `eula.txt`, sets up RCON in `server.properties`, then starts Java.
- **CLI:** `gst` uses the Docker Go SDK to create and control the containers. It only touches containers it created, which it marks with the label `gst-managed=true`.
- **Storage:** each server gets a volume named `gst-<serverName>-data`, mounted at `/data`.
- **Ports:** Minecraft listens on port 25565 inside the container. `-port` chooses the port on your machine.
- **RCON (remote console):** with `-rcon`, `gst` generates a random password, enables RCON on port 25575, and can send console commands to the server with `gst cmd`. You never have to see or type the password. The RCON port is published only on `127.0.0.1`, on a free port Docker picks, so only your machine can reach it and several RCON servers can run at once.

## Usage

### Requirements

- Docker
- Go 1.24 or newer

### 1. Build the server image

From the project root:

```sh
docker build -t gst-minecraft .
```

Rebuild the image whenever `Dockerfile` or `entrypoint.sh` changes. Existing servers keep the image they were created with, so recreate them to pick up the changes (their world data is kept).

### 2. Build the CLI

```sh
go build -o bin/gst ./src
```

The examples below assume `gst` is on your `PATH`. Otherwise, run it as `./bin/gst`.

### 3. Create and run a server

```sh
gst create -eula survival     # create and start the server (accepting the Minecraft EULA is required)
gst list                      # see your servers and their state
```

`create` starts the server right away, so there's no need to run `gst start`. Connect from Minecraft to `localhost:25565`. The first start takes a minute while the world is generated.

If the server can't start, for example because its port is already used by another server, `create` removes the container again and reports the error. Give each server its own `-port`.

`create` options:

| Flag | Meaning | Default |
|---|---|---|
| `-eula` | Accept the [Minecraft EULA](https://aka.ms/MinecraftEULA) (required) | |
| `-port N` | Port on your machine | `25565` |
| `-memory SIZE` | Server memory, `1G` to `32G` | `4G` |
| `-rcon` | Enable RCON so `gst cmd` works | off |

Flags go before the server name:

```sh
gst create -eula -port 25570 -memory 8G -rcon creative
```

### 4. Send console commands (RCON)

Servers created with `-rcon` accept console commands while running. The first argument is the server name, and everything after it is the command:

```sh
gst cmd creative say hello everyone
gst cmd creative time set day
gst cmd creative list
```

### 5. Stop, restart and delete

```sh
gst stop survival                  # graceful stop, waits up to 60 seconds
gst start survival                 # start a stopped server again
gst delete survival                # remove the container, keep the world
gst delete -delvolume survival     # remove the container and its world data
```

A server created again with the same name reuses its old world, unless it was deleted with `-delvolume`.

Run `gst help` for the full list of commands.

## Development

Run the tests from the project root:

```sh
go test ./src
```

The tests don't need Docker. They cover the CLI's argument checks, the Minecraft container settings, the error messages, and `entrypoint.sh` (run against a fake `java`). Because they run `entrypoint.sh` with `sh`, they need Linux, macOS or WSL.

## Known limitations

- Only Minecraft is supported so far.
- `gst` has been tested with Docker Engine on Linux (inside WSL). Docker Desktop should work but hasn't been tested yet.
- `gst` finds Docker through `DOCKER_HOST` or the default socket, and doesn't read Docker contexts. With Docker Desktop on Linux, set `DOCKER_HOST=unix://$HOME/.docker/desktop/docker.sock` first.
