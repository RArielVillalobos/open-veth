FROM debian:bookworm

# Install all apt packages in a single layer
RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iputils-ping \
    traceroute \
    net-tools \
    tcpdump \
    curl \
    wget \
    netcat-openbsd \
    dnsutils \
    iptables \
    apache2-utils \
    siege \
    iperf3 \
    hping3 \
    stress-ng \
    bash \
    python3 \
    python3-pip \
    jq \
    bc \
    vim \
    nano \
    less \
    procps \
    htop \
    openssh-server \
    build-essential \
    libssl-dev \
    git \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Configure SSH — allow root login
RUN mkdir -p /run/sshd \
    && echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config \
    && echo 'root:openveth' | chpasswd

# Build wrk from source (not available as apt package in Bookworm)
RUN git clone --depth 1 --recurse-submodules https://github.com/wg/wrk.git /tmp/wrk \
    && cd /tmp/wrk && make \
    && mv /tmp/wrk/wrk /usr/local/bin/wrk \
    && rm -rf /tmp/wrk

# Download k6
ARG K6_VERSION=0.54.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/grafana/k6/releases/download/v${K6_VERSION}/k6-v${K6_VERSION}-linux-${ARCH}.tar.gz" \
        -O /tmp/k6.tar.gz && \
    tar -xzf /tmp/k6.tar.gz -C /tmp && \
    mv "/tmp/k6-v${K6_VERSION}-linux-${ARCH}/k6" /usr/local/bin/k6 && \
    rm -rf /tmp/k6*

# Download vegeta
ARG VEGETA_VERSION=12.12.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/tsenart/vegeta/releases/download/v${VEGETA_VERSION}/vegeta_${VEGETA_VERSION}_linux_${ARCH}.tar.gz" \
        -O /tmp/vegeta.tar.gz && \
    tar -xzf /tmp/vegeta.tar.gz -C /tmp && \
    mv /tmp/vegeta /usr/local/bin/vegeta && \
    rm -rf /tmp/vegeta*

# Install locust (Python load testing framework with web UI)
RUN pip3 install --break-system-packages locust

EXPOSE 22

CMD ["/bin/bash", "-c", "/usr/sbin/sshd && sleep infinity"]
