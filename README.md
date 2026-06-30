# Warren — P2P Network Operating System

**The decentralized internet layer for Crovi, Thump, and beyond.**

Warren is a unified platform that combines four independent network technologies into a single, resilient P2P infrastructure:

1. **ISP Marketplace** — Trade bandwidth peer-to-peer (Stake-Based Decentralized ISP)
2. **Privacy Gateway** — Policy-transparent, data-encrypted routing (Privacy-First DPN)
3. **Independence Logger** — Cryptographic proof of censorship evasion (Proof-of-Independence)
4. **Satellite Fallback** — Automatic mesh/satellite failover when ground internet fails

---

## Quick Start

### For Users

```bash
# Install Warren daemon
curl -fsSL https://get.warren.crode.net | bash

# Initialize your node
warren init --mode [marketplace|privacy|resilience|all]

# Join the network
warren start

# Monitor your contribution
warren status
```

### For Developers

```bash
git clone https://github.com/croc100/warren.git
cd warren
make build
./warren --help
```

---

## Architecture at a Glance

```
┌──────────────────────────────────────────────────────┐
│           WARREN P2P CORE PROTOCOL                   │
│  - Mesh Routing (adaptive, low-latency)              │
│  - Blockchain Integration (Ethereum/Solana)          │
│  - Smart Contracts (resource allocation, rewards)    │
│  - DNS/NAT Traversal (auto-discovery)                │
└──────────────────────────────────────────────────────┘
    ↓              ↓               ↓              ↓
[Module 1]    [Module 2]      [Module 3]     [Module 4]
[ISP          [Privacy       [Independence   [Satellite
 Market]      Gateway]       Logger]         Fallback]
```

### Module 1: ISP Marketplace

Individuals list spare bandwidth; others purchase encrypted access.

**Key Files:**
- `modules/isp_marketplace/` — Node operator, transaction processing, reputation system
- `contracts/isp_marketplace.sol` — Smart contract for bandwidth trading

**CLI:**
```bash
warren marketplace list --bandwidth 50Mbps --price 0.50/GB
warren marketplace buy --from <node-id> --quantity 10GB
```

---

### Module 2: Privacy Gateway (DPN)

Intent-based routing: policy is public, data is encrypted.

**Key Files:**
- `modules/privacy_gateway/` — Route selection, policy engine, encryption
- `protocol/dpn.md` — DPN specification

**CLI:**
```bash
warren privacy policy set --category news,education,medical --encrypt true
warren privacy route --source <ip> --destination <ip> --policy news
```

---

### Module 3: Independence Logger

Cryptographic proof of censorship evasion, recorded on-chain.

**Key Files:**
- `modules/independence_logger/` — ZKP generation, blockchain logging
- `analytics/` — Real-time censorship metrics dashboard

**CLI:**
```bash
warren independence prove --event "evaded-firewall" --location china
warren independence stats --region asia --timeframe 7d
```

---

### Module 4: Satellite Fallback

Automatic rerouting via satellite + mesh when primary connection fails.

**Key Files:**
- `modules/satellite_fallback/` — Failover logic, LoRa relay incentives
- `integrations/starlink.go` — Starlink API adapter
- `integrations/kuiper.go` — Amazon Kuiper adapter

**CLI:**
```bash
warren satellite enable --priority [starlink|kuiper|loramesh]
warren satellite status
warren lora register --coverage-radius 5km --uptime-sla 99.5
```

---

## Integration with CRODE Ecosystem

### Crovi (Secure No-Log VDI)

```
User → Crovi VDI Login
       ↓
   Warren Privacy Gateway (auto-encrypt desktop traffic)
       ↓
   Warren ISP Marketplace (route via cheapest + fastest node)
       ↓
   GPU Cluster (via encrypted RDMA tunnel)
```

### Thump (Infrastructure Protection)

```
Workload Failover Triggered
       ↓
   Thump: Migrate workload to new node
       ↓
   Warren: Auto-provision new network path
       ↓
   Zero downtime, encrypted traffic preserved
```

---

## Configuration

All Warren config is YAML-based:

```yaml
warren:
  node_id: "node-abc123xyz"
  
  modules:
    isp_marketplace:
      enabled: true
      bandwidth_mbps: 50
      price_per_gb: 0.50
    
    privacy_gateway:
      enabled: true
      policy:
        transparent_categories: [news, education, medical]
        encrypt_all_data: true
    
    independence_logger:
      enabled: true
      blockchain: ethereum  # or solana
      contract_address: "0x..."
    
    satellite_fallback:
      enabled: true
      primary: starlink
      fallback: [kuiper, loramesh]
      auto_failover_delay_ms: 3000
  
  crovi_integration:
    enabled: true
    endpoint: "crovi.crode.net"
    api_key: "$CROVI_API_KEY"
  
  thump_integration:
    enabled: true
    endpoint: "thump.crode.net"
    webhook_secret: "$THUMP_WEBHOOK"
```

---

## Installation & Deployment

### System Requirements

- Linux (x86_64 or ARM64)
- 2GB RAM minimum
- 10Gbps network interface (or WiFi 6E)
- 100GB storage (for node relay cache)

### Supported Platforms

- **Server:** Ubuntu 22.04 LTS, RHEL 8+, Debian 12+
- **Edge:** Raspberry Pi 4+, Banana Pi, Orange Pi
- **Smart Router:** OpenWrt 22.03+, DD-WRT

### From Binary

```bash
# Latest stable
wget https://releases.warren.crode.net/warren-1.0.0-linux-amd64.tar.gz
tar xzf warren-*.tar.gz
sudo ./install.sh

# Or via package manager
sudo apt install warren  # Debian/Ubuntu
sudo dnf install warren  # RHEL/Fedora
```

### From Source

```bash
git clone https://github.com/croc100/warren.git
cd warren

# Build all modules
make all

# Or build specific module
make MODULE=isp_marketplace

# Run tests
make test

# Deploy locally
make deploy-local
```

---

## Roadmap

| Phase | Timeline | Scope |
|-------|----------|-------|
| **Alpha** | Months 1–3 | ISP Marketplace MVP (testnet) |
| **Beta** | Months 4–6 | + Privacy Gateway, Blockchain logging |
| **Release Candidate** | Months 7–9 | + Independence Logger analytics |
| **General Availability** | Months 10–12 | + Satellite Fallback, Crovi/Thump integration |
| **Horizontal Expansion** | Year 2+ | Warren DNS, Warren Email, Warren Storage |

---

## Community & Support

- **Documentation:** https://docs.warren.crode.net
- **GitHub Issues:** https://github.com/croc100/warren/issues
- **Discord:** https://discord.gg/warren-crode
- **Email:** support@warren.crode.net

---

## License

Warren is licensed under **AGPL-3.0** (with commercial licensing available).

See [LICENSE](LICENSE) for details.

---

## Contributors

- croc100 (Founder)
- See [CONTRIBUTORS.md](CONTRIBUTORS.md) for full list

---

## Citation

If you reference Warren in academic work:

```bibtex
@software{warren2026,
  title={Warren: P2P Network Operating System},
  author={croc100},
  year={2026},
  url={https://github.com/croc100/warren}
}
```

---

**Warren: Internet freedom, without compromise.**