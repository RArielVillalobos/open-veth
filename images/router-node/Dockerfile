FROM frrouting/frr:latest

# Install additional network utilities
RUN apk add --no-cache \
    frr \
    iproute2 \
    iputils \
    net-tools \
    tcpdump \
    bash

# Enable essential FRR routing daemons
# Zebra is mandatory for kernel interaction
# OSPF and BGP are enabled by default for lab usage
RUN sed -i 's/zebra=no/zebra=yes/' /etc/frr/daemons && \
    sed -i 's/ospfd=no/ospfd=yes/' /etc/frr/daemons && \
    sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons && \
    sed -i 's/staticd=no/staticd=yes/' /etc/frr/daemons

# Expose routing ports for monitoring
EXPOSE 179 2601 2604

# Use official FRR entrypoint to manage services
ENTRYPOINT ["/usr/lib/frr/docker-start"]
