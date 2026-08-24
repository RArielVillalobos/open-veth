#!/usr/bin/env python3
"""
Create 8 loop device nodes starting at the first free slot reported by the kernel.
This avoids colliding with host loop devices already in use (e.g. snapd mounts).
Requires: /dev/loop-control mapped into the container, CAP_MKNOD capability.
"""
import fcntl
import os

LOOP_CTL_GET_FREE = 0x4C82
LOOP_MAJOR = 7
SLOTS = 8

try:
    with open("/dev/loop-control", "rb") as lc:
        first_free = fcntl.ioctl(lc, LOOP_CTL_GET_FREE, 0)
except OSError as e:
    print(f"storage-init: cannot open /dev/loop-control: {e}")
    raise SystemExit(1)

for n in range(first_free, first_free + SLOTS):
    path = f"/dev/loop{n}"
    if not os.path.exists(path):
        try:
            os.mknod(path, 0o660, os.makedev(LOOP_MAJOR, n))
            print(f"storage-init: created {path}")
        except OSError as e:
            print(f"storage-init: mknod {path} failed: {e}")
    else:
        print(f"storage-init: {path} already exists, skipping")
