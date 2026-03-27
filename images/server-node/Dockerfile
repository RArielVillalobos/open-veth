FROM debian:bookworm

# Install systemd and remove unnecessary units that fail in containers
RUN apt-get update && apt-get install -y --no-install-recommends \
    systemd \
    systemd-sysv \
    dbus \
    && rm -rf /var/lib/apt/lists/*

# Remove units that don't make sense in a container
RUN find /lib/systemd/system/sysinit.target.wants/ -not -name systemd-tmpfiles-setup.service -type f -delete \
    && rm -f /lib/systemd/system/multi-user.target.wants/* \
    && rm -f /etc/systemd/system/*.wants/* \
    && rm -f /lib/systemd/system/local-fs.target.wants/* \
    && rm -f /lib/systemd/system/sockets.target.wants/*udev* \
    && rm -f /lib/systemd/system/sockets.target.wants/*initctl* \
    && rm -f /lib/systemd/system/basic.target.wants/* \
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

# Install scripting and automation tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    python3 \
    python3-pip \
    expect \
    git \
    jq \
    tree \
    bc \
    && rm -rf /var/lib/apt/lists/*

# Install system administration tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    procps \
    htop \
    lsof \
    strace \
    sudo \
    rsyslog \
    logrotate \
    less \
    man-db \
    manpages \
    manpages-es \
    bash-completion \
    locales \
    && rm -rf /var/lib/apt/lists/*

# Install services (not enabled — student does it manually)
RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-server \
    nginx \
    apache2 \
    dnsmasq \
    chrony \
    vsftpd \
    nfs-kernel-server \
    nfs-common \
    vim \
    nano \
    && rm -rf /var/lib/apt/lists/*

# Configure locale
RUN sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
    && sed -i '/es_ES.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen
ENV LANG=es_ES.UTF-8

# Generate SSH host keys
RUN ssh-keygen -A

# Configure SSH — allow root login
RUN echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
RUN echo 'root:openveth' | chpasswd

# Disable all services by default — student enables them manually with systemctl
RUN systemctl disable nginx apache2 dnsmasq chrony vsftpd || true

# Enable only SSH so the student can connect to manage the node
RUN systemctl enable ssh

EXPOSE 22

# systemd as PID 1
CMD ["/sbin/init"]
