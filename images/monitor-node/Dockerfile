FROM openveth/server:latest

# Install Grafana via direct .deb download (avoids apt repo GPG issues in containers)
ARG GRAFANA_VERSION=10.4.3
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://dl.grafana.com/oss/release/grafana_${GRAFANA_VERSION}_${ARCH}.deb" \
        -O /tmp/grafana.deb && \
    apt-get update && \
    apt-get install -y /tmp/grafana.deb && \
    rm /tmp/grafana.deb && \
    rm -rf /var/lib/apt/lists/*

# Download Prometheus binary
ARG PROMETHEUS_VERSION=2.54.1
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/prometheus/prometheus/releases/download/v${PROMETHEUS_VERSION}/prometheus-${PROMETHEUS_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/prometheus.tar.gz && \
    tar -xzf /tmp/prometheus.tar.gz -C /tmp && \
    mv "/tmp/prometheus-${PROMETHEUS_VERSION}.linux-${ARCH}/prometheus" /usr/local/bin/prometheus && \
    mv "/tmp/prometheus-${PROMETHEUS_VERSION}.linux-${ARCH}/promtool" /usr/local/bin/promtool && \
    rm -rf /tmp/prometheus*

# Download node_exporter — runs on this node AND available to copy to lab nodes
ARG NODE_EXPORTER_VERSION=1.8.2
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/prometheus/node_exporter/releases/download/v${NODE_EXPORTER_VERSION}/node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/node_exporter.tar.gz && \
    tar -xzf /tmp/node_exporter.tar.gz -C /tmp && \
    mv "/tmp/node_exporter-${NODE_EXPORTER_VERSION}.linux-${ARCH}/node_exporter" /usr/local/bin/node_exporter && \
    rm -rf /tmp/node_exporter*

# Download blackbox_exporter to /opt/exporters/
ARG BLACKBOX_EXPORTER_VERSION=0.25.0
RUN ARCH=$(dpkg --print-architecture) && \
    mkdir -p /opt/exporters && \
    wget -q "https://github.com/prometheus/blackbox_exporter/releases/download/v${BLACKBOX_EXPORTER_VERSION}/blackbox_exporter-${BLACKBOX_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/blackbox_exporter.tar.gz && \
    tar -xzf /tmp/blackbox_exporter.tar.gz -C /tmp && \
    mv "/tmp/blackbox_exporter-${BLACKBOX_EXPORTER_VERSION}.linux-${ARCH}/blackbox_exporter" /opt/exporters/blackbox_exporter && \
    cp /usr/local/bin/node_exporter /opt/exporters/node_exporter && \
    chmod +x /opt/exporters/* && \
    rm -rf /tmp/blackbox_exporter*

# Install SNMP client tools
RUN apt-get update && apt-get install -y --no-install-recommends snmp && \
    rm -rf /var/lib/apt/lists/*

# Download snmp_exporter
ARG SNMP_EXPORTER_VERSION=0.26.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/prometheus/snmp_exporter/releases/download/v${SNMP_EXPORTER_VERSION}/snmp_exporter-${SNMP_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/snmp_exporter.tar.gz && \
    tar -xzf /tmp/snmp_exporter.tar.gz -C /tmp && \
    mv "/tmp/snmp_exporter-${SNMP_EXPORTER_VERSION}.linux-${ARCH}/snmp_exporter" /usr/local/bin/snmp_exporter && \
    mkdir -p /etc/snmp_exporter && \
    mv "/tmp/snmp_exporter-${SNMP_EXPORTER_VERSION}.linux-${ARCH}/snmp.yml" /etc/snmp_exporter/snmp.yml && \
    chmod +x /usr/local/bin/snmp_exporter && \
    rm -rf /tmp/snmp_exporter*

# Configure Prometheus
RUN mkdir -p /etc/prometheus /var/lib/prometheus/data

SHELL ["/bin/bash", "-c"]

RUN cat > /etc/prometheus/prometheus.yml <<'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node_exporter'
    static_configs:
      - targets: ['localhost:9100']

  # To scrape your lab nodes, install node_exporter on them and add targets here:
  # - job_name: 'lab_nodes'
  #   static_configs:
  #     - targets:
  #         - '<NODE_IP>:9100'
EOF

# Set Grafana credentials via config file (admin/admin, no forced password change)
RUN cat >> /etc/grafana/grafana.ini <<'EOF'

[security]
admin_user = admin
admin_password = admin
EOF

# Create Prometheus systemd service
RUN cat > /etc/systemd/system/prometheus.service <<'EOF'
[Unit]
Description=Prometheus Monitoring System
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/prometheus \
  --config.file=/etc/prometheus/prometheus.yml \
  --storage.tsdb.path=/var/lib/prometheus/data \
  --web.listen-address=:9090
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# Create node_exporter systemd service
RUN cat > /etc/systemd/system/node_exporter.service <<'EOF'
[Unit]
Description=Prometheus Node Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/node_exporter
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# Create snmp_exporter systemd service
RUN cat > /etc/systemd/system/snmp_exporter.service <<'EOF'
[Unit]
Description=Prometheus SNMP Exporter
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/snmp_exporter --config.file=/etc/snmp_exporter/snmp.yml
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# Enable services — start automatically when the container boots
RUN systemctl enable grafana-server && \
    systemctl enable prometheus && \
    systemctl enable node_exporter && \
    systemctl enable snmp_exporter

EXPOSE 3000 9090

CMD ["/sbin/init"]
