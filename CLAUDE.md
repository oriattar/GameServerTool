# CLAUDE.md

`gst` is a Go CLI that creates and manages game servers in Docker containers through the Docker Go SDK (`github.com/moby/moby/client`). Only Minecraft is supported so far. See README.md for user-facing usage.

This is a learning project: the owner is learning Go and Docker. When they ask how something works, explain it with a short example rather than editing their code. Many changes are made by the owner between turns, so re-read a file before editing it.

## Commands

All commands run from the project root.

- Build the CLI: `go build -o bin/gst ./src`
- Run the tests: `go test ./src` (no Docker needed)
- Vet: `go vet ./src`
- Build the server image: `docker build -t gst-minecraft .`

`src/` is a single `package main` inside the module `github.com/oriattar/gst`. Run the package (`go run ./src`), never a single file.

## Layout

- `src/main.go`: argument routing, one `handle<Command>` function per command, flag parsing, `helpText`
- `src/commands.go`: the Docker operations behind each command, and `rconCommand`
- `src/rcon.go`: the RCON protocol (packet building, reading and login)
- `src/games.go`: per-game container settings (`constructMinecraftServerData`)
- `src/*_test.go`: tests next to the code they test
- `Dockerfile`, `entrypoint.sh`: the `gst-minecraft` image
- `compose.yaml`: a manual test setup, not used by the CLI

## Conventions

### Error handling

- Errors are logged only in the `handle<Command>` functions in `main.go`, using `log.Fatal(err)`. Every other function returns the error.
- Wrap with a short, lowercase description of what failed, and use `%w`: `fmt.Errorf("inspecting container: %w", err)`. No trailing `\n` or period.
- Top-level operations name the action and the server: `starting server %s: %w`. Inner steps describe their step: `reading packet size: %w`. Don't wrap where it only repeats the caller's context (for example `readInt32`, `interact`).
- Wrong-state conditions are errors too: `server %s is not running`.

### CLI

- Each command has a `handle<Command>` function in `main.go`.
- Commands with flags use their own `flag.NewFlagSet`. Flags must come before positional arguments. Validate the positional count with `NArg()`.
- Usage and validation errors print a usage line and exit with code 1, before any Docker call.
- When a command or flag changes, update its usage line, `helpText` (including Examples), the tests, and README.md.

### Style

- Constants are `ALL_CAPS` (`SERVER_NAME`, `DEFAULT_TIMEOUT`).
- Keep functions short and comments sparse, matching the existing code.

## Docker details

- Containers created by gst carry the label `gst-managed=true`. Commands refuse to touch containers without it.
- Each server's world is in the volume `gst-<name>-data`, mounted at `/data`. It also carries `gst-managed=true`.
- Minecraft always listens on 25565 inside the container. `-port` only changes the host side of the mapping.
- With `-rcon`, `create` generates a password with `crypto/rand` and passes it as the env vars `RCON=true` and `RCON_PASSWORD`. `getServerInfo` reads it back from the container's config, so the user never types it. RCON listens on 25575, and `rconCommand` connects to the container's bridge network IP.
- The image copies `entrypoint.sh` in at build time. After changing it, rebuild the image and recreate the containers; the tests run the local file, so they can pass while the image is stale.

## Tests

- `commands_test.go` builds the real binary once (`TestMain` in `main_test.go`) and runs it with bad arguments, and with `DOCKER_HOST` pointed at a closed port to check error messages.
- `games_test.go` runs `entrypoint.sh` with a fake `java` on `PATH`.
- Commands that need a real Docker daemon (successful create, start, stop, delete, cmd) are not covered.
