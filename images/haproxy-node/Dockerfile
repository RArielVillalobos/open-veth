FROM alpine:latest

RUN apk add --no-cache \
    haproxy \
    openssh \
    iproute2 \
    iputils \
    net-tools \
    tcpdump \
    curl \
    wget \
    iptables \
    bash \
    vim \
    nano

# Configure SSH
RUN ssh-keygen -A \
    && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
    && echo 'root:openveth' | chpasswd

# Default haproxy config: stats on 8404, example frontend/backend commented out
RUN mkdir -p /etc/haproxy && cat > /etc/haproxy/haproxy.cfg <<'EOF'
global
    log stdout format raw local0
    maxconn 1024

defaults
    log     global
    mode    http
    option  httplog
    timeout connect 5s
    timeout client  30s
    timeout server  30s

# Stats page — available at http://<host-ip>:<mapped-port>/stats
frontend stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 5s
    stats show-legends
    no log

# ── Example: HTTP load balancer ──────────────────────────────────────────────
# Uncomment and adapt to your topology.
#
# frontend http_front
#     bind *:80
#     default_backend web_servers
#
# backend web_servers
#     balance roundrobin
#     option httpchk GET /
#     server web1 <IP1>:80 check
#     server web2 <IP2>:80 check
#     server web3 <IP3>:80 check
EOF

# Entrypoint: start haproxy with default config, then sshd as PID 1
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22 8404

CMD ["/entrypoint.sh"]
