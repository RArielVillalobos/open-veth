FROM debian:bookworm-slim

# Enable man pages (slim images exclude them by default)
# Also reinstall base packages so their man pages are included
RUN sed -i '/usr\/share\/man/d' /etc/dpkg/dpkg.cfg.d/docker \
    && apt-get update \
    && apt-get install -y --reinstall coreutils findutils grep sed gzip \
    && rm -rf /var/lib/apt/lists/*

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
    # System utilities
    procps \
    sudo \
    file \
    tree \
    zip \
    unzip \
    jq \
    bc \
    man-db \
    manpages \
    manpages-es \
    less \
    bash-completion \
    locales \
    netcat-openbsd \
    dnsutils \
    # Editors & tools
    vim \
    nano \
    git \
    # SSH
    openssh-client \
    openssh-server \
    # Web server
    nginx \
    cron \
    && rm -rf /var/lib/apt/lists/*

# Configure locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
    && sed -i '/es_ES.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen
ENV LANG=es_ES.UTF-8

# Configure SSH
RUN mkdir -p /run/sshd
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

EXPOSE 22

# Start SSH daemon and keep container alive
CMD ["/bin/bash", "-c", "/usr/sbin/sshd && sleep infinity"]
