#!/bin/sh
# Start haproxy with default config (stats on :8404)
haproxy -f /etc/haproxy/haproxy.cfg -D -p /var/run/haproxy.pid 2>/dev/null || echo "WARNING: haproxy failed to start"
# Start sshd as PID 1
exec /usr/sbin/sshd -D
