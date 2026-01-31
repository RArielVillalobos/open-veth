# OpenVeth

**OpenVeth** is a modern, minimalist, web-based network emulator. Built with Go and Angular, it serves as a lightweight alternative to legacy tools like GNS3 or EVE-NG, leveraging native Linux features for high performance.

![Architecture](https://img.shields.io/badge/Architecture-Microservices-blue)
![Backend](https://img.shields.io/badge/Backend-Go-cyan)
![Frontend](https://img.shields.io/badge/Frontend-Angular-red)
![License](https://img.shields.io/badge/License-AGPL%20v3-red)

## 🚀 Features

- **Web-Based Canvas:** Design network topologies with a modern Drag & Drop interface (Cytoscape.js).
- **Real Emulation:** Nodes are real Docker containers (Alpine, FRRouting).
- **In-Browser Terminal:** Access consoles via WebSockets + xterm.js directly from the UI.
- **Native Performance:** Uses Linux `veth pairs` and Bridges, no heavy VMs required.
- **Instant Deployment:** Go-based orchestrator deploys complex labs in seconds.

## 🛠️ Architecture

- **Frontend:** Angular 18+, TailwindCSS, SignalStore, Cytoscape.js.
- **Backend:** Go (Gin Framework), Docker SDK, Netlink.
- **Infrastructure:** Docker Compose for development.

## 🏁 Getting Started

### Prerequisites
- Docker & Docker Compose
- Linux Environment (Native or WSL2)

### Quick Start
1. **Initialize Environment:**
   ```bash
   make dev-env    # Starts Docker infrastructure
   make images     # Builds node images (Host & Router)
   ```

2. **Run Backend:**
   ```bash
   make run-api
   ```

3. **Run Frontend:**
   ```bash
   # In a new terminal
   make run-ui
   ```
   Open `http://localhost:4200` in your browser.

## 📄 License

This project is licensed under the **GNU Affero General Public License v3 (AGPLv3)**. See the [LICENSE](./LICENSE) file for details.

---
*OpenVeth - Virtual Ethernet Network Emulator*
