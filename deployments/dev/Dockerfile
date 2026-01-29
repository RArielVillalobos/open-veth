# Usamos una imagen base de Go reciente
FROM golang:1.23-alpine

# Instalamos herramientas necesarias para el sistema y redes
# iproute2: ip, ss, bridge, etc.
# docker-cli: para interactuar con el daemon del host
# build-base: gcc para cgo si fuera necesario
RUN apk add --no-cache \
    iproute2 \
    iptables \
    iputils \
    docker-cli \
    build-base \
    git \
    bash

# Directorio de trabajo
WORKDIR /app

# Mantenemos el contenedor vivo para desarrollo interactivo
CMD ["sleep", "infinity"]
