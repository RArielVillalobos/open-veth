FROM frrouting/frr:latest

# Instalamos utilidades básicas de red adicionales
RUN apk add --no-cache iproute2 iputils tcpdump bash nano

# Habilitar demonios por defecto
# Zebra es obligatorio (gestiona el kernel)
# OSPF y BGP suelen ser los más usados en laboratorios
RUN sed -i 's/zebra=no/zebra=yes/' /etc/frr/daemons && \
    sed -i 's/ospfd=no/ospfd=yes/' /etc/frr/daemons && \
    sed -i 's/bgpd=no/bgpd=yes/' /etc/frr/daemons && \
    sed -i 's/staticd=no/staticd=yes/' /etc/frr/daemons

# Exponer puertos típicos de routing por si se necesita monitorizar
EXPOSE 179 2601 2604

# El punto de entrada oficial de FRR ya maneja el arranque de los servicios
# Solo nos aseguramos de que el contenedor no muera
ENTRYPOINT ["/usr/lib/frr/docker-start"]
