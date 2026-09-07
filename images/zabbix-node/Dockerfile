FROM openveth/base:latest

ARG DEBIAN_FRONTEND=noninteractive

# Runtime dependencies: MariaDB, Apache2, PHP
RUN apt-get update && apt-get install -y --no-install-recommends \
    mariadb-server \
    apache2 \
    libapache2-mod-php \
    php-mysql \
    php-mbstring \
    php-gd \
    php-xml \
    php-bcmath \
    php-ldap \
    php-zip \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Zabbix 8.0 official repository (Debian 12) + packages
RUN curl -fsSL https://repo.zabbix.com/zabbix/8.0/release/debian/pool/main/z/zabbix-release/zabbix-release_latest_8.0+debian12_all.deb \
    -o /tmp/zabbix-release.deb \
    && dpkg -i /tmp/zabbix-release.deb \
    && rm /tmp/zabbix-release.deb \
    && apt-get update && apt-get install -y --no-install-recommends \
    zabbix-server-mysql \
    zabbix-frontend-php \
    zabbix-apache-conf \
    zabbix-sql-scripts \
    zabbix-agent2 \
    && rm -rf /var/lib/apt/lists/*

# Set PHP timezone (required by Zabbix frontend, avoids setup wizard warning)
RUN find /etc/php -name php.ini -path '*/apache2/*' \
    -exec sed -i 's|;date.timezone =|date.timezone = UTC|' {} +

# First-boot: initialize MariaDB and Zabbix DB + web config (runs once via flag file)
COPY zabbix-init.sh /usr/local/sbin/zabbix-init.sh
COPY zabbix-init.service /lib/systemd/system/zabbix-init.service
RUN chmod +x /usr/local/sbin/zabbix-init.sh \
    && systemctl enable mariadb apache2 zabbix-server zabbix-agent2 zabbix-init

EXPOSE 22 80 10051

CMD ["/sbin/init"]
