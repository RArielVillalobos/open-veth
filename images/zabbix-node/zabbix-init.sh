#!/bin/bash
# First-boot: initialize MariaDB and Zabbix database + web config.
# Subsequent boots are skipped via flag file (also guarded by ConditionPathExists in the unit).
set -eo pipefail

FLAG=/var/lib/zabbix/.initialized
[ -f "$FLAG" ] && exit 0

echo "zabbix-init: waiting for MariaDB to be ready..."
for i in $(seq 1 30); do
    mysqladmin ping --silent 2>/dev/null && break
    sleep 1
done
if ! mysqladmin ping --silent 2>/dev/null; then
    echo "zabbix-init: MariaDB not ready after 30s, aborting" >&2
    exit 1
fi

echo "zabbix-init: creating database and user..."
mysql -u root <<'SQL'
CREATE DATABASE IF NOT EXISTS zabbix CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE USER IF NOT EXISTS 'zabbix'@'localhost' IDENTIFIED BY 'zabbix';
GRANT ALL PRIVILEGES ON zabbix.* TO 'zabbix'@'localhost';
FLUSH PRIVILEGES;
SQL

echo "zabbix-init: importing schema (this takes ~30s on first boot)..."
TABLES=$(mysql -uzabbix -pzabbix zabbix -sNe \
    "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='zabbix';" 2>/dev/null || echo 0)
if [ "$TABLES" -lt 10 ]; then
    zcat /usr/share/zabbix/sql-scripts/mysql/server.sql.gz | mysql -uzabbix -pzabbix zabbix
    TABLES=$(mysql -uzabbix -pzabbix zabbix -sNe \
        "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='zabbix';" 2>/dev/null || echo 0)
    if [ "$TABLES" -lt 10 ]; then
        echo "zabbix-init: schema import failed (got $TABLES tables), not marking as initialized" >&2
        exit 1
    fi
fi

echo "zabbix-init: configuring zabbix_server.conf..."
sed -i 's/^# DBPassword=/DBPassword=zabbix/' /etc/zabbix/zabbix_server.conf

echo "zabbix-init: writing web frontend config..."
mkdir -p /etc/zabbix/web
cat > /etc/zabbix/web/zabbix.conf.php <<'PHP'
<?php
$DB['TYPE']           = 'MYSQL';
$DB['SERVER']         = 'localhost';
$DB['PORT']           = '0';
$DB['DATABASE']       = 'zabbix';
$DB['USER']           = 'zabbix';
$DB['PASSWORD']       = 'zabbix';
$DB['SCHEMA']         = '';
$DB['ENCRYPTION']     = false;
$DB['KEY_FILE']       = '';
$DB['CERT_FILE']      = '';
$DB['CA_FILE']        = '';
$DB['VERIFY_HOST']    = false;
$DB['CIPHER_LIST']    = '';
$DB['DOUBLE_IEEE754'] = true;
$ZBX_SERVER           = 'localhost';
$ZBX_SERVER_PORT      = '10051';
$ZBX_SERVER_NAME      = 'OpenVeth Zabbix';
$IMAGE_FORMAT_DEFAULT = IMAGE_FORMAT_PNG;
PHP

chown www-data:www-data /etc/zabbix/web/zabbix.conf.php

mkdir -p /var/lib/zabbix
touch "$FLAG"
echo "zabbix-init: done. Login at http://<node-ip>/ — user: Admin  password: zabbix"
