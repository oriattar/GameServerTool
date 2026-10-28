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

MEMORY="${MEMORY:-1G}"
exec java -Xms"$MEMORY" -Xmx"$MEMORY" -jar /opt/minecraft/server.jar nogui
