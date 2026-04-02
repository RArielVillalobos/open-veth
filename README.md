# OpenVeth

**OpenVeth** is a web-based network emulator powered by real Linux kernel networking (namespaces, veth pairs, bridges) and Docker. Design topologies visually, access node terminals in the browser, capture packets live, benchmark services under load, and built-in observability — no VMs, no cost.

![Backend](https://img.shields.io/badge/Backend-Go_1.23-00ADD8?logo=go)
![Frontend](https://img.shields.io/badge/Frontend-Angular_21-DD0031?logo=angular)
![Docker](https://img.shields.io/badge/Docker-Required-2496ED?logo=docker)
![License](https://img.shields.io/badge/License-AGPL%20v3-green)

---

<p align="center">
  <img src="https://i.imgur.com/Lx55pqp.gif" alt="OpenVeth Demo - Creating network topologies in real-time" width="800">
</p>

<p align="center">
  <i>Create network topologies visually, connect to nodes via terminal, capture packets in real-time.</i>
</p>

---

### Broadcast & Collision Domain Overlay

Visualize **broadcast** and **collision** domains directly on the topology canvas with a single click.

<p align="center">
  <img src="https://i.imgur.com/fPzXKFJ.png" alt="Broadcast domains highlighted on the topology" width="800">
</p>
<p align="center"><i>Broadcast domains — each colored region is an independent broadcast domain separated by routers.</i></p>

<p align="center">
  <img src="https://i.imgur.com/yVI4b5O.png" alt="Collision domains highlighted on the topology" width="800">
</p>
<p align="center"><i>Collision domains — note how the hub merges all connected ports into a single shared collision domain, while the switch isolates each link.</i></p>

---

### Visual Traceroute

Run traceroute from any node and watch the path light up on the graph in real-time.

<p align="center">
  <img src="https://i.imgur.com/8sBFlCP.gif" alt="Visual traceroute - path highlighted on topology" width="800">
</p>
<p align="center"><i>Each hop is numbered on the canvas. The result table shows IP, RTT, and the resolved node name.</i></p>

<p align="center">
  <img src="https://i.imgur.com/dKg5Otm.png" alt="Traceroute result dialog" width="800">
</p>

---

## Features

| Feature | Description |
|---------|-------------|
| **Visual Topology Builder** | Drag-and-drop canvas (Cytoscape.js) to design network labs |
| **Integrated Terminals** | Shell access to any node (bash, vtysh) via xterm.js |
| **Live Packet Capture** | Wireshark-style traffic sniffing in real-time |
| **Dynamic Routing** | Full FRRouting support (OSPF, BGP, IS-IS, Static) |
| **SSH Between Nodes** | Connect from one node to another within the lab |
| **DHCP Server** | Automatic IP assignment for realistic LAN labs |
| **Infrastructure as Code** | Export/import topologies as YAML — includes node positions, links, IP configs, and static routes |
| **Lab Management** | Create, save, and switch between lab projects |
| **State Persistence** | IP/route configs auto-saved every 30s. Manual Save also snapshots the full filesystem of each node (`docker commit`) — surviving lab switches and restarts |
| **Broadcast/Collision Domain Overlay** | Visualize broadcast and collision domains highlighted on the topology canvas |
| **Visual Traceroute** | Run traceroute from any node and see the path highlighted on the topology graph |
| **Cloud Gateway** | Automatic NAT gateway — IP forwarding and masquerade pre-configured, connect lab nodes to real internet |
| **Link Toggle** | Enable or disable any link without deleting it — right-click a cable and select Disable/Enable |
| **Node Power Control** | Power off and on any node — right-click a node and select Power Off/On. The UI updates in real-time, including when a node is shut down from within (e.g. `systemctl poweroff` on a SERVER node) |

---

## Node Types

| Type | Base Image | Use Case |
|------|------------|----------|
| **HOST** | Alpine Linux | End devices, servers, clients |
| **ROUTER** | FRRouting | Dynamic routing, NAT, firewalls |
| **SWITCH** | Linux Bridge | L2 switching, broadcast domains |
| **HUB** | Linux Bridge (no MAC learning) | L1 repeater, floods all traffic to all ports |
| **CLOUD** | Alpine Linux | NAT gateway — automatically configures IP forwarding and masquerade for internet access |
| **LINUX** | Debian bookworm-slim | Scripting and automation — bash, python3, cron, apt, git, jq, tree, vim, nano, netcat, dig, man pages (Spanish), bash-completion, sudo, direct internet access |
| **SERVER** | Debian bookworm + systemd | Sysadmin and services labs — systemd as PID 1, nginx, apache2, dnsmasq, chrony, vsftpd, nfs-kernel-server, nfs-common pre-installed. Manage services with `systemctl`, inspect logs with `journalctl`. Direct internet access |
| **MONITOR** | Debian bookworm + systemd | Observability labs — Grafana + Prometheus + snmp_exporter pre-installed and auto-started. Loki is installed but disabled by default — enable with `systemctl enable loki --now`. Ports 3000 (Grafana) and 9090 (Prometheus) auto-mapped. Exporters available in `/opt/exporters/` to copy to any node via `scp`: `node_exporter`, `nginx_exporter`, `apache_exporter`, `promtail`, `frr_exporter` (BGP/OSPF metrics from FRRouting), and more |
| **TESTER** | Debian bookworm | Load and stress testing — `wrk`, `k6`, `siege`, `ab`, `iperf3`, `hping3`, `vegeta`, `stress-ng`, `locust` pre-installed. Use `tc netem` to simulate network degradation |

All nodes include: `iproute2`, `tcpdump`, `ping`, `traceroute`, `curl`

---

## Quick Start

### Requirements

| Component | Version | Notes |
|-----------|---------|-------|
| Docker | 20.10+ | Required |
| Docker Compose | 2.0+ | Required |
| Make | - | Required |
| Linux / WSL2 | - | Netlink operations require Linux kernel |

> **Windows Users**: OpenVeth requires Linux networking. Docker Desktop must be configured to use WSL2 as the backend (this is the default in recent versions). Once Docker is running, use `start.bat` as described below.

### Installation

**Linux / macOS:**
```bash
git clone https://github.com/RArielVillalobos/open-veth.git
cd open-veth
make up
```

**Windows (Docker Desktop with WSL2):**
```
1. Clone the repository
2. Double-click start.bat
```

> `start.bat` verifies that Docker is running and that the WSL2 backend is active before starting. If either check fails, it shows a clear error message with instructions.

That's it! The command will:
- Pull node images from Docker Hub (Host, Router, Linux, Server, Monitor, Tester — Switch, Hub, and Cloud reuse the Host image)
- Auto-detect available ports (avoids conflicts with existing services)
- Start the application and show you the URL to open in your browser

```
========================================
  OpenVeth listo!
  Abri tu browser en:
  http://localhost:80
========================================
```

### Credentials

| Access | URL / Command | User | Password |
|--------|--------------|------|----------|
| OpenVeth UI | `http://localhost` | — | — |
| Node terminal (SSH) | `ssh root@<node-ip>` | `root` | `openveth` |
| Grafana (MONITOR node) | `http://localhost:<mapped-port>` | `admin` | `admin` |

> The Grafana port is auto-assigned — find it in the node card after activating a lab with a MONITOR node.

### Useful Commands

```bash
make help         # Show all available commands
make up           # Start OpenVeth (auto-detects ports)
make down         # Stop OpenVeth
make logs         # View container logs
make status       # Show current ports and container status
make reset-ports  # Force port re-detection on next start
```

### Development Setup

For local development (requires Go 1.23+ and Node.js 22+):

```bash
# Start dev container (provides Linux networking tools)
make dev-env

# In another terminal, enter the container
docker exec -it openveth-dev bash

# Inside the container, run the API
go run ./cmd/openveth-api

# Run frontend natively (terminal 2, on your host)
cd frontend && npm install && npm start
# → UI at http://localhost:4200
```

---

## Architecture

OpenVeth translates your visual topology into real Linux kernel networking structures.

```mermaid
graph TD
    Browser[Browser] <-->|HTTP / WebSocket| Frontend[Angular + Cytoscape.js]
    Frontend <-->|REST API| Backend[Go + Gin]

    Backend --> Docker[Docker SDK]
    Backend --> Netlink[Netlink API]
    Backend --> DB[(SQLite / PostgreSQL)]

    Docker --> Containers
    Netlink --> Veth[Veth Pairs]

    subgraph Containers [Network Nodes]
        H[HOST - Alpine]
        R[ROUTER - FRR]
        S[SWITCH - Bridge]
        HB[HUB - Repeater]
        L[LINUX - Debian]
        SRV[SERVER - Debian+systemd]
        MON[MONITOR - Grafana+Prometheus]
        TST[TESTER - wrk/k6/siege/iperf3]
    end

    Veth -.->|connects| Containers

    C[CLOUD - Internet GW] -.->|Docker bridge| Internet((Internet))
```

### How it works

1. **Nodes** → Docker containers with isolated network namespaces (HOST, ROUTER, SWITCH, CLOUD)
2. **Links** → `veth` pairs connecting container namespaces
3. **SWITCH nodes** → Have a Linux bridge (`br0`) inside for L2 switching
4. **HUB nodes** → Same as SWITCH but with MAC learning disabled (floods all frames)
5. **CLOUD nodes** → Keep `eth0` connected to the Docker bridge. On creation, automatically enable IP forwarding and set up `iptables MASQUERADE` so connected lab nodes can reach real internet
6. **Links** → Can be enabled/disabled at runtime (right-click → Disable/Enable) without deleting them, simulating link failures
7. **Node power** → Nodes can be powered off and on (right-click → Power Off/On). The backend listens to Docker events so the UI updates even when a node shuts itself down from within (e.g. `systemctl poweroff`)
8. **Startup reconciliation** → On restart, the backend rebuilds containers, links, and IP configs automatically from the database
9. **Filesystem snapshots** → The Save button runs `docker commit` on every running node, capturing the full filesystem state (configs in `/etc`, installed packages, etc.). On next lab activation, each node boots from its snapshot instead of the base image

---

### Cloud Gateway — Internet Access for Lab Nodes

The **CLOUD** node acts as a NAT gateway. When created, it automatically enables IP forwarding and sets up masquerade rules so any connected lab node can reach real internet.

**Example topology:**
```
HOST ── ROUTER ── CLOUD ── Internet
```

**Configuration steps:**

On **CLOUD** (terminal):
```bash
ip addr add 10.0.2.2/30 dev eth1
ip link set eth1 up
ip route add 10.0.1.0/30 via 10.0.2.1   # return route per lab subnet
```

On **ROUTER** (terminal):
```bash
ip addr add 10.0.1.1/30 dev eth1
ip addr add 10.0.2.1/30 dev eth2
ip link set eth1 up && ip link set eth2 up
ip route add default via 10.0.2.2
sysctl -w net.ipv4.ip_forward=1
```

On **HOST** (terminal):
```bash
ip addr add 10.0.1.2/30 dev eth1
ip link set eth1 up
ip route add default via 10.0.1.1
ping 8.8.8.8   # should work
```

### Topology YAML Format

Export and import full lab configurations, including IP addresses and static routes:

```yaml
name: My Lab
nodes:
  - name: R1
    type: router
    x: 200
    y: 150
    interfaces:
      - interface: eth1
        address: 10.0.1.1/30
      - interface: eth2
        address: 10.0.2.1/30
    routes:
      - dst: 10.0.3.0/24
        gateway: 10.0.2.2
        dev: eth2
  - name: HOST-1
    type: host
    x: 400
    y: 150
    interfaces:
      - interface: eth1
        address: 10.0.1.2/30
    routes:
      - dst: 0.0.0.0/0
        gateway: 10.0.1.1
        dev: eth1
links:
  - source: R1
    target: HOST-1
    source_int: eth1
    target_int: eth1
    enabled: true
```

On import, IP addresses and routes are applied to the containers immediately and persisted to the database so they survive restarts.

### Link Toggle — Simulate Link Failures

Right-click any cable on the canvas and select **Disable Link** to bring the interfaces down without deleting the connection. Re-enable it at any time. Useful for testing routing protocol convergence (OSPF/BGP failover), STP, and redundancy scenarios.

### Node Power Control — Simulate Node Failures

Right-click any node and select **Power Off** to stop the container without deleting it. The node turns transparent on the canvas and all its links become inactive. Select **Power On** to restart it.

This is distinct from disconnecting a cable:

| | Disable Link | Power Off |
|---|---|---|
| Node stays running | ✅ | ❌ |
| Affects | One interface | All interfaces |
| Services (nginx, sshd...) | Active | Stopped |
| Routing protocols | Partial reconvergence | Full session drop |
| Analogy | Unplug a cable | Cut the power |

The UI updates in real-time even when the node shuts itself down from within — for example, running `systemctl poweroff` inside a SERVER node triggers an immediate visual update via the Docker event stream. Useful for testing OSPF/BGP reconvergence, high-availability setups, and service recovery scenarios.

---

## Why OpenVeth?

| Feature | OpenVeth | GNS3 | EVE-NG | Packet Tracer |
|---------|:--------:|:----:|:------:|:-------------:|
| Real Linux networking | ✅ | ✅ | ✅ | ❌ |
| Web-based UI | ✅ | ❌ | ✅ | ❌ |
| Low resource usage | ✅ | ❌ | ❌ | ✅ |
| No VMs required | ✅ | ❌ | ❌ | ✅ |
| Free & Open Source | ✅ | ✅ | ⚠️ | ❌ |
| FRRouting support | ✅ | ✅ | ✅ | ❌ |
| Live packet capture | ✅ | ✅ | ✅ | ❌ |
| SSH between nodes | ✅ | ✅ | ✅ | ❌ |
| L2 switching / L1 hub | ✅ | ✅ | ✅ | ✅ |
| Internet connectivity | ✅ | ✅ | ✅ | ❌ |
| Broadcast/collision domain overlay | ✅ | ❌ | ❌ | ❌ |
| Visual traceroute | ✅ | ❌ | ❌ | ❌ |
| Node power off/on | ✅ | ✅ | ✅ | ❌ |
| Grafana + Prometheus built-in | ✅ | ❌ | ❌ | ❌ |
| Load testing node built-in | ✅ | ❌ | ❌ | ❌ |
| SNMP monitoring | ✅ | ✅ | ✅ | ❌ |

---

## Use Cases

- **Networking Courses**: Hands-on labs for CCNA, Network+, Linux networking
- **Protocol Analysis**: Capture and analyze OSPF, BGP, ARP, DHCP packets with live packet capture
- **Network Troubleshooting**: Visualize broadcast/collision domains and trace packet paths interactively
- **Network Administration**: Practice SSH, remote management, and troubleshooting
- **System Administration**: Practice service management with systemd, configure nginx/apache2/dnsmasq/NFS, manage users and firewall rules
- **Scripting & Automation**: Write bash scripts, schedule cron jobs, automate network configuration across multiple nodes
- **Observability**: Deploy a Monitor node with Grafana + Prometheus + Loki pre-configured. Copy exporters from `/opt/exporters/` to any lab node via `scp` and correlate metrics and logs in real-time
- **Load & Stress Testing**: Deploy a Tester node with `wrk`, `k6`, `siege`, `ab`, `iperf3`, `hping3`, `vegeta`, `stress-ng` and `locust` pre-installed. Generate HTTP load against a Server node, measure TCP/UDP throughput with `iperf3`, simulate degraded network conditions with `tc netem`, and observe the impact in real-time on the Monitor node
- **Infrastructure Design**: Prototype topologies before production deployment

---

## Contributing

Contributions are welcome! Please follow these steps:

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

This project is licensed under the **GNU Affero General Public License v3 (AGPLv3)**.

See the [LICENSE](LICENSE) file for details.

---