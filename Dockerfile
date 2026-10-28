FROM eclipse-temurin:25-jre


ARG SERVER_JAR=https://piston-data.mojang.com/v1/objects/33680f5f2ac32864d6d7cf5e56a705fdb3e05f4c/server.jar
ADD ${SERVER_JAR} /opt/minecraft/server.jar


ENV MEMORY=4G
ENV EULA=FALSE

COPY --chmod=755 entrypoint.sh /entrypoint.sh

WORKDIR /data
VOLUME /data
EXPOSE 25565

ENTRYPOINT ["/entrypoint.sh"]
