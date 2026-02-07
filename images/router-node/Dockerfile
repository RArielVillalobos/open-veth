FROM frrouting/frr:latest

# Install essential networking services and utilities
RUN apk add --no-cache \
    frr \
    iproute2 \
    iputils \
    net-tools \
    tcpdump \
    dnsmasq \
    iptables \
    openssh \
    bash \
    nano \
    traceroute \
    bind-tools

# Configure SSH
RUN ssh-keygen -A
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

# Enable essential FRR routing daemons
RUN sed -i 's/zebra=no/zebra=yes/' /etc/frr/daemons && \
    sed -i 's/ospfd=no/ospfd=yes/' /etc/frr/daemons && \
    sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons && \
    sed -i 's/staticd=no/staticd=yes/' /etc/frr/daemons

# Expose ports (SSH + Routing)
EXPOSE 22 179 2601 2604

# Custom entrypoint to start SSH and then the official FRR start script
ENTRYPOINT ["/bin/sh", "-c", "/usr/sbin/sshd && /usr/lib/frr/docker-start"]
