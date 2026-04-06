FROM debian:bookworm

# Install systemd and remove unnecessary units that fail in containers
RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd \
    systemd-sysv \
    dbus \
    && rm -rf /var/lib/apt/lists/*

RUN find /lib/systemd/system/sysinit.target.wants/ -not -name systemd-tmpfiles-setup.service -type f -delete \
    && rm -f /lib/systemd/system/multi-user.target.wants/* \
    && rm -f /etc/systemd/system/*.wants/* \
    && rm -f /lib/systemd/system/local-fs.target.wants/* \
    && rm -f /lib/systemd/system/sockets.target.wants/*udev* \
    && rm -f /lib/systemd/system/sockets.target.wants/*initctl* \
    && rm -f /lib/systemd/system/basic.target.wants/* \
    && ln -sf /lib/systemd/system/systemd-user-sessions.service /lib/systemd/system/multi-user.target.wants/systemd-user-sessions.service \
    || true

# Install networking tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    iproute2 \
    iputils-ping \
    traceroute \
    net-tools \
    tcpdump \
    curl \
    wget \
    iptables \
    ufw \
    netcat-openbsd \
    dnsutils \
    && rm -rf /var/lib/apt/lists/*

# Install scripting and general-purpose tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    python3 \
    python3-pip \
    expect \
    git \
    jq \
    tree \
    bc \
    procps \
    htop \
    lsof \
    strace \
    sudo \
    less \
    man-db \
    manpages \
    manpages-es \
    bash-completion \
    locales \
    vim \
    nano \
    && rm -rf /var/lib/apt/lists/*

# Install SSH
RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-server \
    && rm -rf /var/lib/apt/lists/*

# Configure locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
    && sed -i '/es_ES.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen
ENV LANG=es_ES.UTF-8

# Generate SSH host keys and configure root login
RUN ssh-keygen -A
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd
RUN systemctl enable ssh

EXPOSE 22

CMD ["/sbin/init"]
