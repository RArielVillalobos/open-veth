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
    rm -rf /tmp/blackbox_exporter*

# Download nginx-prometheus-exporter to /opt/exporters/
ARG NGINX_EXPORTER_VERSION=1.3.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/nginx/nginx-prometheus-exporter/releases/download/v${NGINX_EXPORTER_VERSION}/nginx-prometheus-exporter_${NGINX_EXPORTER_VERSION}_linux_${ARCH}.tar.gz" \
        -O /tmp/nginx_exporter.tar.gz && \
    tar -xzf /tmp/nginx_exporter.tar.gz -C /tmp && \
    mv /tmp/nginx-prometheus-exporter /opt/exporters/nginx_exporter && \
    rm -rf /tmp/nginx_exporter*

# Download postgres_exporter to /opt/exporters/
ARG POSTGRES_EXPORTER_VERSION=0.15.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/prometheus-community/postgres_exporter/releases/download/v${POSTGRES_EXPORTER_VERSION}/postgres_exporter-${POSTGRES_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/postgres_exporter.tar.gz && \
    tar -xzf /tmp/postgres_exporter.tar.gz -C /tmp && \
    mv "/tmp/postgres_exporter-${POSTGRES_EXPORTER_VERSION}.linux-${ARCH}/postgres_exporter" /opt/exporters/postgres_exporter && \
    rm -rf /tmp/postgres_exporter*

# Download mysqld_exporter to /opt/exporters/
ARG MYSQL_EXPORTER_VERSION=0.15.1
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/prometheus/mysqld_exporter/releases/download/v${MYSQL_EXPORTER_VERSION}/mysqld_exporter-${MYSQL_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/mysql_exporter.tar.gz && \
    tar -xzf /tmp/mysql_exporter.tar.gz -C /tmp && \
    mv "/tmp/mysqld_exporter-${MYSQL_EXPORTER_VERSION}.linux-${ARCH}/mysqld_exporter" /opt/exporters/mysql_exporter && \
    rm -rf /tmp/mysql_exporter*

# Download redis_exporter to /opt/exporters/
ARG REDIS_EXPORTER_VERSION=1.62.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/oliver006/redis_exporter/releases/download/v${REDIS_EXPORTER_VERSION}/redis_exporter-v${REDIS_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/redis_exporter.tar.gz && \
    tar -xzf /tmp/redis_exporter.tar.gz -C /tmp && \
    mv "/tmp/redis_exporter-v${REDIS_EXPORTER_VERSION}.linux-${ARCH}/redis_exporter" /opt/exporters/redis_exporter && \
    rm -rf /tmp/redis_exporter*

# Download Promtail to /opt/exporters/ — agent to ship logs to Loki (copy to lab nodes)
ARG PROMTAIL_VERSION=3.0.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/grafana/loki/releases/download/v${PROMTAIL_VERSION}/promtail-linux-${ARCH}.zip" \
        -O /tmp/promtail.zip && \
    apt-get update && apt-get install -y --no-install-recommends unzip && \
    unzip -q /tmp/promtail.zip -d /tmp && \
    mv "/tmp/promtail-linux-${ARCH}" /opt/exporters/promtail && \
    rm -rf /tmp/promtail* && \
    rm -rf /var/lib/apt/lists/*

# Download apache_exporter to /opt/exporters/
ARG APACHE_EXPORTER_VERSION=1.0.8
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/Lusitaniae/apache_exporter/releases/download/v${APACHE_EXPORTER_VERSION}/apache_exporter-${APACHE_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/apache_exporter.tar.gz && \
    tar -xzf /tmp/apache_exporter.tar.gz -C /tmp && \
    mv "/tmp/apache_exporter-${APACHE_EXPORTER_VERSION}.linux-${ARCH}/apache_exporter" /opt/exporters/apache_exporter && \
    rm -rf /tmp/apache_exporter*

# Download frr_exporter to /opt/exporters/ — copy to router nodes to expose BGP/OSPF metrics
ARG FRR_EXPORTER_VERSION=1.10.1
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/tynany/frr_exporter/releases/download/v${FRR_EXPORTER_VERSION}/frr_exporter-${FRR_EXPORTER_VERSION}.linux-${ARCH}.tar.gz" \
        -O /tmp/frr_exporter.tar.gz && \
    tar -xzf /tmp/frr_exporter.tar.gz -C /tmp && \
    mv "/tmp/frr_exporter-${FRR_EXPORTER_VERSION}.linux-${ARCH}/frr_exporter" /opt/exporters/frr_exporter && \
    rm -rf /tmp/frr_exporter*

# Fix permissions on all exporters
RUN chmod +x /opt/exporters/*

# Download Loki binary — log aggregation backend
ARG LOKI_VERSION=3.0.0
RUN ARCH=$(dpkg --print-architecture) && \
    wget -q "https://github.com/grafana/loki/releases/download/v${LOKI_VERSION}/loki-linux-${ARCH}.zip" \
        -O /tmp/loki.zip && \
    apt-get update && apt-get install -y --no-install-recommends unzip && \
    unzip -q /tmp/loki.zip -d /tmp && \
    mv "/tmp/loki-linux-${ARCH}" /usr/local/bin/loki && \
    chmod +x /usr/local/bin/loki && \
    rm -rf /tmp/loki* && \
    rm -rf /var/lib/apt/lists/*

# Configure Loki
RUN mkdir -p /etc/loki /var/lib/loki

RUN cat > /etc/loki/loki.yml <<'EOF'
auth_enabled: false

server:
  http_listen_port: 3100

common:
  path_prefix: /var/lib/loki
  storage:
    filesystem:
      chunks_directory: /var/lib/loki/chunks
      rules_directory: /var/lib/loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h
EOF

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

# Create Loki systemd service
RUN cat > /etc/systemd/system/loki.service <<'EOF'
[Unit]
Description=Grafana Loki Log Aggregation
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/loki --config.file=/etc/loki/loki.yml
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

# Enable services — start automatically when the container boots
RUN systemctl enable grafana-server && \
    systemctl enable prometheus && \
    systemctl enable node_exporter && \
    systemctl enable snmp_exporter && \
    systemctl enable loki

EXPOSE 3000 9090

CMD ["/sbin/init"]
