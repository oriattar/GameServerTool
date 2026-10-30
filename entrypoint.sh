#!/bin/sh
set -e

case "$EULA" in
  TRUE|true|True) ;;
  *)
    echo "You have not accepted the Minecraft EULA. Please set -e EULA=TRUE." >&2
    exit 1
    ;;
esac

echo "eula=true" > eula.txt

# set_property KEY VALUE: sets KEY in server.properties, replacing any existing value
set_property() {
  if [ -f server.properties ]; then
    grep -v "^$1=" server.properties > server.properties.tmp || true
    mv server.properties.tmp server.properties
  fi
  printf '%s=%s\n' "$1" "$2" >> server.properties
}

case "$RCON" in
  TRUE|true|True)
    if [ -z "$RCON_PASSWORD" ]; then
      echo "RCON is enabled but RCON_PASSWORD is not set. Please set -e RCON_PASSWORD=<password>." >&2
      exit 1
    fi
    set_property enable-rcon true
    set_property rcon.password "$RCON_PASSWORD"
    set_property rcon.port "${RCON_PORT:-25575}"
    ;;
  *)
    set_property enable-rcon false
    ;;
esac

if [ -z "$MEMORY" ]; then
  echo "MEMORY is not set. Please set -e MEMORY=<1G to 32G>." >&2
  exit 1
fi

MEMORY_NUM="${MEMORY%G}" # strip the trailing G, e.g. 4G -> 4
case "$MEMORY_NUM" in
  ''|*[!0-9]*) MEMORY_NUM=0 ;; # no trailing G or not a whole number, fails the range check below
esac
if [ "$MEMORY_NUM" = "$MEMORY" ] || [ "$MEMORY_NUM" -lt 1 ] || [ "$MEMORY_NUM" -gt 32 ]; then
  echo "Invalid MEMORY value '$MEMORY', expected 1G to 32G." >&2
  exit 1
fi

exec java -Xms"$MEMORY" -Xmx"$MEMORY" -jar /opt/minecraft/server.jar nogui
