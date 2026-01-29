# OpenVeth

**OpenVeth** is a lightweight, modern, web-based network emulator. Designed as a fast alternative to legacy tools like GNS3 or EVE-NG, it allows users to design, deploy, and manage complex network topologies directly from their browser.

![Architecture](https://img.shields.io/badge/Architecture-Microservices-blue)
![Backend](https://img.shields.io/badge/Backend-Go-cyan)
![Frontend](https://img.shields.io/badge/Frontend-Angular-red)
![Docker](https://img.shields.io/badge/Containerization-Docker-blue)
![License](https://img.shields.io/badge/License-AGPL%20v3-red)

## 🚀 Key Features

- **Lightweight Architecture:** Powered by Docker containers (Alpine Linux, FRRouting) instead of heavy Virtual Machines.
- **Real Network Plumbing:** Uses native `veth pairs` and `Linux Bridges` for high-performance packet switching.
- **Web-First Experience:** Modern Angular interface with an interactive topology canvas.
- **In-Browser Terminal:** Real-time console access to routers and hosts via WebSockets and xterm.js.
- **Professional Routing:** Router nodes equipped with **FRRouting** (Cisco-like CLI) supporting OSPF, BGP, and more.

## 🛠️ Technical Overview

The project consists of two main components:

1.  **Backend (Go):** 
    - REST API & WebSockets.
    - Orchestrator interfacing with the Docker Daemon.
    - Linux Kernel manipulation (Netlink) for virtual cabling.
2.  **Frontend (Angular):**
    - Single Page Application (SPA).
    - Topology visualization engine.
    - Integrated terminal management.

## 🏁 Getting Started

To set up the development environment, build the base images, and run the API server, please refer to the detailed guide:

👉 **[Development Guide (DEVELOPMENT.md)](./DEVELOPMENT.md)**

### Core Requirements
- Docker & Docker Compose
- Linux Environment (Native or WSL2)
- Go 1.23+

## 📄 License

This project is licensed under the **GNU Affero General Public License v3 (AGPLv3)**. See the [LICENSE](./LICENSE) file for details.

---
*OpenVeth - Virtual Ethernet Network Emulator*