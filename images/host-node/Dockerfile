FROM alpine:latest

# Instalar herramientas de PC "Standard"
# openssh: Para conectarse remotamente
# iputils: Para ping
# curl/wget: Para simular tráfico web
# tcpdump: Para analizar tráfico
# bash: Para una terminal cómoda
# vim/nano: Para editar archivos
RUN apk add --no-cache \
    openssh \
    iproute2 \
    iputils \
    curl \
    wget \
    tcpdump \
    bash \
    vim \
    nano

# Configurar SSH
# Generar host keys
RUN ssh-keygen -A
# Permitir root login (SOLO PARA LABS)
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
# Establecer password root: "openveth"
RUN echo 'root:openveth' | chpasswd

# Exponer puerto 22
EXPOSE 22

# Al arrancar, iniciamos SSHD en segundo plano y nos quedamos en sleep
CMD ["/bin/sh", "-c", "/usr/sbin/sshd && sleep infinity"]
