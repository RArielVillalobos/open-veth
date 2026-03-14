FROM debian:bookworm-slim

# Install networking tools + scripting essentials
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Networking
    iproute2 \
    iputils-ping \
    traceroute \
    net-tools \
    tcpdump \
    curl \
    wget \
    iptables \
    # Scripting
    bash \
    python3 \
    python3-pip \
    # Utilities
    openssh-client \
    openssh-server \
    cron \
    vim \
    nano \
    git \
    && rm -rf /var/lib/apt/lists/*

# Configure SSH
RUN mkdir -p /run/sshd
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

EXPOSE 22

# Start SSH daemon and keep container alive
CMD ["/bin/bash", "-c", "/usr/sbin/sshd && sleep infinity"]
