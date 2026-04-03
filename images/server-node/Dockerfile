FROM openveth/base:latest

# Install server services and admin tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-server \
    nginx \
    apache2 \
    dnsmasq \
    chrony \
    vsftpd \
    nfs-kernel-server \
    nfs-common \
    rsyslog \
    logrotate \
    && rm -rf /var/lib/apt/lists/*

# Replace apache2 service file — PrivateTmp=true fails in Docker on Linux (exit 226/NAMESPACE)
RUN cat > /lib/systemd/system/apache2.service <<'EOF'
[Unit]
Description=The Apache HTTP Server
After=network.target remote-fs.target nss-lookup.target
Documentation=https://httpd.apache.org/docs/2.4/

[Service]
Type=forking
Environment=APACHE_STARTED_BY_SYSTEMD=true
ExecStart=/usr/sbin/apachectl start
ExecStop=/usr/sbin/apachectl graceful-stop
ExecReload=/usr/sbin/apachectl graceful
KillMode=mixed
Restart=on-abort
OOMPolicy=continue

[Install]
WantedBy=multi-user.target
EOF

# Replace chrony service file — many namespace sandbox directives fail in Docker on Linux
RUN cat > /lib/systemd/system/chrony.service <<'EOF'
[Unit]
Description=chrony, an NTP client/server
Documentation=man:chronyd(8) man:chronyc(1) man:chrony.conf(5)
After=network.target
ConditionCapability=CAP_SYS_TIME

[Service]
Type=forking
PIDFile=/run/chrony/chronyd.pid
EnvironmentFile=-/etc/default/chrony
User=_chrony
ExecStart=!/usr/sbin/chronyd $DAEMON_OPTS
RuntimeDirectory=chrony
RuntimeDirectoryMode=0700
RuntimeDirectoryPreserve=restart
StateDirectory=chrony
StateDirectoryMode=0750
LogsDirectory=chrony
LogsDirectoryMode=0750

[Install]
Alias=chronyd.service
WantedBy=multi-user.target
EOF

# Disable all services by default — student enables them manually with systemctl
RUN systemctl disable nginx apache2 dnsmasq chrony vsftpd || true

EXPOSE 22

CMD ["/sbin/init"]
