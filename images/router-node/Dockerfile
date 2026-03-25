FROM debian:bookworm-slim

# Install prerequisites for FRR repo
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    gnupg \
    lsb-release \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Add official FRRouting repository
RUN curl -s https://deb.frrouting.org/frr/keys.gpg \
        | tee /usr/share/keyrings/frrouting.gpg > /dev/null && \
    echo "deb [signed-by=/usr/share/keyrings/frrouting.gpg] https://deb.frrouting.org/frr bookworm frr-stable" \
        > /etc/apt/sources.list.d/frr.list

# Install FRRouting
RUN apt-get update && apt-get install -y --no-install-recommends \
    frr \
    frr-pythontools \
    && rm -rf /var/lib/apt/lists/*

# Install networking tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iputils-ping \
    traceroute \
    net-tools \
    tcpdump \
    wget \
    iptables \
    dnsmasq \
    dnsutils \
    netcat-openbsd \
    bash \
    nano \
    vim \
    procps \
    && rm -rf /var/lib/apt/lists/*

# Install SNMP
RUN apt-get update && apt-get install -y --no-install-recommends \
    snmpd \
    snmp \
    && rm -rf /var/lib/apt/lists/*

# Install SSH
RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-server \
    && rm -rf /var/lib/apt/lists/*

# Configure SSH
RUN mkdir -p /run/sshd && ssh-keygen -A
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

# Enable FRR daemons
RUN sed -i 's/zebra=no/zebra=yes/' /etc/frr/daemons && \
    sed -i 's/ospfd=no/ospfd=yes/' /etc/frr/daemons && \
    sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons && \
    sed -i 's/staticd=no/staticd=yes/' /etc/frr/daemons

# Configure SNMP — read-only community "public"
RUN echo "rocommunity public" > /etc/snmp/snmpd.conf && \
    echo "agentAddress udp:161,udp6:[::1]:161" >> /etc/snmp/snmpd.conf

EXPOSE 22 161/udp 179 2601 2604

# Start SSH, SNMP and FRR
CMD ["/bin/bash", "-c", "/usr/sbin/sshd; snmpd; /usr/lib/frr/frrinit.sh start; sleep infinity"]
