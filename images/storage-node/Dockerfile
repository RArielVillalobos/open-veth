FROM openveth/base:latest

RUN apt-get update && apt-get install -y --no-install-recommends \
    fdisk \
    parted \
    gdisk \
    dosfstools \
    e2fsprogs \
    xfsprogs \
    btrfs-progs \
    bsdextrautils \
    lvm2 \
    mdadm \
    cryptsetup \
    nfs-common \
    smartmontools \
    testdisk \
    && rm -rf /var/lib/apt/lists/*

# Startup script: allocates 8 loop device nodes starting at the first kernel-free slot,
# so they never collide with host loop devices already in use (e.g. snap mounts).
COPY storage-init.py /usr/local/sbin/storage-init.py
COPY storage-init.service /lib/systemd/system/storage-init.service
RUN systemctl enable storage-init.service

EXPOSE 22

CMD ["/sbin/init"]
