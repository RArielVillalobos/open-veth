FROM alpine:latest

# Install standard networking utilities
# openssh: For remote access labs
# iputils: For ping and tracepath
# curl/wget: For web traffic simulation
# tcpdump: For packet analysis
# iptables: For firewall and NAT experiments
# bash: For a better shell experience
# vim/nano: For configuration file editing
RUN apk add --no-cache \
    openssh \
    iproute2 \
    iputils \
    net-tools \
    curl \
    wget \
    tcpdump \
    iptables \
    bash \
    vim \
    nano

# Configure SSH
RUN ssh-keygen -A
# Allow root login for lab purposes
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
# Default password: "openveth"
RUN echo 'root:openveth' | chpasswd

EXPOSE 22

# Start SSH daemon and keep container alive
CMD ["/bin/sh", "-c", "/usr/sbin/sshd && sleep infinity"]
