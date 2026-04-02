FROM debian:bookworm-slim

# Install bridge utilities, SNMP daemon, and SSH for terminal access
RUN apt-get update && apt-get install -y --no-install-recommends \
    bridge-utils \
    iproute2 \
    snmpd \
    snmp \
    openssh-server \
    procps \
    tcpdump \
    vim \
    nano \
    && rm -rf /var/lib/apt/lists/*

# Configure SSH — allow root login for lab purposes
RUN mkdir -p /run/sshd && ssh-keygen -A
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

# Copy pre-configured SNMP config
COPY snmpd.conf /etc/snmp/snmpd.conf

EXPOSE 22 161/udp

# Start SSH and SNMP, keep container alive
CMD ["/bin/bash", "-c", "/usr/sbin/sshd; snmpd; sleep infinity"]
