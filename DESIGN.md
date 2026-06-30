# Warren Technical Architecture & Design

## Table of Contents

1. [Core Protocol](#core-protocol)
2. [Module Specifications](#module-specifications)
3. [Integration Layer](#integration-layer)
4. [Data Flow](#data-flow)
5. [Security Model](#security-model)
6. [Implementation Roadmap](#implementation-roadmap)

---

## Core Protocol

### Warren Protocol Fundamentals

Warren is built on a **multi-layered P2P routing protocol** that sits between Layer 3 (IP) and Layer 4 (Transport):

```
Application Layer (VPN, DNS, Email)
          ↓
Warren Control Plane (Mesh Discovery, Resource Market, Blockchain)
          ↓
Warren Data Plane (Encrypted tunnels, Smart routing)
          ↓
IP Layer (Actual internet routing)
```

### Key Properties

| Property | Implementation |
|----------|----------------|
| **Discovery** | DHT (Distributed Hash Table) + mDNS for local mesh |
| **Routing** | Adaptive pathing (Dijkstra variant, latency-optimized) |
| **Encryption** | ChaCha20-Poly1305 (default), AES-GCM option |
| **NAT Traversal** | STUN + ICE + UPnP fallback |
| **Incentive** | Tokens (ERC-20 on Ethereum, or native Solana) |
| **Consensus** | Blockchain (Ethereum or Solana) for market settlement |
| **Micro-payments** | **Off-chain Payment Channels for per-packet/per-GB transactions** (eliminate gas fee overhead) |

### Node Types

1. **Routing Node** (runs all the time)
   - Stores DHT entries
   - Relays traffic
   - Announces available capacity

2. **Marketplace Node** (sells bandwidth)
   - Advertising node + routing
   - Tracks bandwidth utilization
   - Publishes pricing

3. **Privacy Node** (runs policy engine)
   - Encryption/decryption
   - Policy decision-making
   - Never stores plaintext user data

4. **Logger Node** (records independence proofs)
   - ZKP verification
   - Blockchain interaction
   - Analytics aggregation

5. **Satellite Node** (failover gateway)
   - Starlink/Kuiper uplink
   - LoRa mesh coordinator
   - Automatic routing failover

---

## Module Specifications

### Module 1: ISP Marketplace

#### Architecture

```
┌─────────────────────────────────────────┐
│      Marketplace Smart Contract         │
│  (Ethereum/Solana)                      │
│  - Bandwidth listing                    │
│  - Transaction settlement               │
│  - Reputation tracking                  │
└─────────────────────────────────────────┘
          ↑                    ↑
    [Seller Node]         [Buyer Node]
    Advertises:           Purchases:
    - Bandwidth           - Encrypted tunnel
    - Price per GB        - Auto-settlement
    - Uptime SLA
```

#### Smart Contract Interface

```solidity
contract WarrenISPMarketplace {
  // Seller registers available bandwidth
  function listBandwidth(
    uint256 bandwidthMbps,
    uint256 pricePerGB,
    uint256 uptimeSLApercent
  ) external;
  
  // Buyer initiates transaction
  function purchaseBandwidth(
    address seller,
    uint256 amountGB,
    uint256 maxPricePerGB,
    bytes32 encryptionKeyHash
  ) external payable;
  
  // Settle after usage
  function settle(
    bytes32 transactionId,
    uint256 actualBytesUsed,
    bytes calldata proof
  ) external;
  
  // Reputation system
  function rateNode(
    address node,
    uint8 rating,
    string calldata comment
  ) external;
}
```

#### Settlement Model: Hybrid On-Chain + Off-Chain

**Problem:** Every settlement on-chain = gas fees can exceed transaction value.

**Solution:** **Off-chain Payment Channels** (similar to Lightning Network):

```
1. Buyer deposits tokens to smart contract (locks collateral)
2. Buyer ↔ Seller exchange SIGNED receipts off-chain (no gas)
3. Every N hours or when dispute occurs: final settlement posted on-chain
4. Result: 1000x reduction in blockchain overhead
```

#### Data Structures

```protobuf
message MarketplaceNode {
  string node_id;
  uint32 bandwidth_mbps;
  float price_per_gb;
  uint8 uptime_sla_percent;
  repeated Rating ratings;
  uint64 reputation_score;
  repeated Transaction transactions;
  string payment_channel_address;  // Off-chain channel identifier
}

message Transaction {
  string transaction_id;
  string seller_id;
  string buyer_id;
  uint32 amount_gb;
  float price_per_gb;
  int64 timestamp;
  TransactionStatus status;
  bytes encryption_key_hash;
  uint64 actual_bytes_used;
}
```

#### Node Implementation (Pseudocode)

```go
type MarketplaceNode struct {
  NodeID           string
  BandwidthMbps    uint32
  PricePerGB       float32
  UptimeSLA        uint8
  ContractAddress  common.Address
  Web3Client       *ethclient.Client
}

func (n *MarketplaceNode) ListBandwidth(ctx context.Context) error {
  // 1. Calculate available bandwidth
  available := n.BandwidthMbps - n.getCurrentUtilization()
  
  // 2. Publish to smart contract
  tx, err := n.Contract.ListBandwidth(
    &bind.TransactOpts{From: n.AccountAddress},
    available,
    n.PricePerGB,
    n.UptimeSLA,
  )
  
  // 3. Announce in DHT
  n.DHT.Announce(fmt.Sprintf("marketplace:%s", n.NodeID))
  
  return err
}

func (n *MarketplaceNode) HandleBandwidthPurchase(
  ctx context.Context,
  buyerID string,
  amountGB uint32,
) (*Tunnel, error) {
  // 1. Verify buyer's payment on-chain
  // 2. Generate encrypted tunnel (ChaCha20-Poly1305)
  // 3. Return tunnel config to buyer
}
```

---

### Module 2: Privacy Gateway (DPN)

#### Design Philosophy

**Goal:** Governments can inspect *intent* (what user is doing), but cannot inspect *data* (specific content). **Protocols themselves must be stealthy.**

```
Without Privacy Gateway (traditional VPN):
  [User] → [Entire packet encrypted] → [VPN Server]
  Result: Government can't see anything → bans it

With Privacy Gateway (Warren):
  [User] → [HTTPS/TLS Disguise Layer] → [Policy: "news"] [Content: encrypted] → [DPN Server]
  Result: Government sees "normal HTTPS web traffic" + doesn't see Warren metadata
```

#### Traffic Obfuscation Layer (Critical for Censorship Evasion)

**Problem:** If Warren packets have recognizable headers/signatures, government DPI systems will blacklist them immediately (by IP/ASN blocking).

**Solution:** **All Warren control plane traffic is obfuscated as standard HTTPS/TLS traffic** (using Pluggable Transports like Tor's obfs4 or Shadowsocks techniques):

- Warren metadata (policy categories, encryption keys, node addresses) is encrypted and wrapped inside TLS handshake mimicry
- To passive observers (governments, ISPs), Warren is indistinguishable from normal HTTPS browsing
- Only endpoints (Warren nodes) can decrypt and interpret the policy metadata
- Active probing (government attempting to connect as a client) fails because no real HTTPS server responds

#### Policy Engine

```yaml
# User's privacy policy (stored locally, never leaves device)
policies:
  news:
    categories: [politics, international, tech, science]
    encryption: required
    transparency: intent_only
  
  education:
    categories: [university, online_courses, research]
    encryption: required
    transparency: metadata_only  # Teacher can see attendance, not content
  
  medical:
    categories: [health, pharmacy, mental_health]
    encryption: required
    transparency: none  # Completely hidden
  
  social:
    categories: [messaging, social_media]
    encryption: optional
    transparency: metadata_only
```

#### DPN Route Selection Algorithm

```
For each packet:
  1. Extract: destination IP, destination port
  2. Classify: which policy category does this belong to?
  3. Determine: transparency level (intent_only, metadata_only, none)
  4. Select route:
     a) If privacy: use Warren ISP Marketplace (cheapest + fastest encrypted path)
     b) If metadata_only: use mixed path (some transparent hops, some encrypted)
     c) If intent_only: use public path + policy header only
  5. Add: encryption wrapper (ChaCha20-Poly1305)
  6. Forward: through selected route
```

#### Data Flow

```protobuf
message PrivacyPacket {
  // Visible to all nodes (for routing)
  string policy_category;         // "news", "education", etc.
  int64 timestamp;
  string source_node_id;
  
  // Encrypted (only destination can decrypt)
  bytes content;                  // Actual user data
  bytes encryption_nonce;
  string encryption_key_hash;     // Matches blockchain record
}
```

#### Implementation (Pseudocode)

```go
type PrivacyGateway struct {
  UserPolicies   map[string]*PolicyRule
  DHT            *dht.DHT
  EncryptionKey  *chacha20poly1305.ChaCha20Poly1305
  RouteCache     *lru.Cache
}

func (pg *PrivacyGateway) ProcessPacket(pkt *Packet) (*RoutedPacket, error) {
  // 1. Classify packet
  category := pg.classifyDestination(pkt.DestIP, pkt.DestPort)
  policy := pg.UserPolicies[category]
  
  // 2. Determine transparency
  transparency := policy.Transparency
  
  // 3. Find best route
  route := pg.selectRoute(category, policy, pkt)
  
  // 4. Encrypt content
  ciphertext, nonce := pg.encrypt(pkt.Payload)
  
  // 5. Create privacy packet
  privacyPkt := &PrivacyPacket{
    PolicyCategory:   category,
    Timestamp:        time.Now().Unix(),
    SourceNodeID:     pg.NodeID,
    Content:          ciphertext,
    EncryptionNonce:  nonce,
    EncryptionKeyHash: hashKey(pg.EncryptionKey),
  }
  
  // 6. Route through selected path
  return &RoutedPacket{
    Route:          route,
    Content:        privacyPkt,
    Transparency:   transparency,
  }, nil
}
```

---

### Module 3: Independence Logger (Proof-of-Independence)

#### Zero-Knowledge Proof (ZKP) Generation

When a user successfully evades censorship, Warren generates a **non-interactive ZKP** proving:
- "I successfully bypassed firewall X"
- WITHOUT revealing: the specific data transmitted, source/destination IPs, exact timestamps

```
Proof Structure:
  commitment = hash(firewall_id, evasion_timestamp, random_nonce)
  witness = (firewall_id, evasion_timestamp, random_nonce)
  proof = zkp_prove(commitment, witness)
```

#### Blockchain Recording with Anonymity Protection

**Problem:** If user wallet address (`msg.sender`) is revealed on-chain, wallet tracking can de-anonymize the user.

**Solution:** **Relayer-Assisted Anonymous Proof Submission**:

- User generates ZKP locally (never broadcasts wallet address)
- User submits proof to Warren's **anonymous Relayer nodes** (multi-hop, encrypted)
- Relayer (trusted Warren infrastructure node) pays gas fee from relayer's own wallet
- On-chain transaction shows: Relayer address → Proof Hash (user wallet is never on-chain)
- Relayer is periodically rotated and collateral-bonded to prevent collusion

```solidity
contract WarrenIndependenceLogger {
  // Relayers are bonded, rotating addresses (prevents tracking)
  mapping(address => uint256) public relayerBond;
  address[] public activeRelayers;
  
  event CensorshipEvaded(
    bytes32 indexed proofHash,
    string region,
    int64 timestamp,
    bytes zkProof,
    address relayerAddress  // NOT user wallet
  );
  
  function logEvasionViaRelayer(
    bytes32 proofHash,
    string region,
    bytes calldata zkProof,
    bytes calldata relayerSignature  // Proves relayer submitted this
  ) external {
    require(verifyRelayerSignature(relayerSignature), "Invalid relayer signature");
    require(verifySingleProof(zkProof, proofHash), "Invalid proof");
    emit CensorshipEvaded(proofHash, region, block.timestamp, zkProof, msg.sender);
  }
  
  function getRegionStats(string region, uint64 timeframe) 
    public view returns (uint256 evasionCount) {
    // Query events filtered by region and timeframe (no wallet linking)
  }
}
```

#### Implementation (Pseudocode)

```go
type IndependenceProof struct {
  ProofID        string
  Region         string
  Timestamp      int64
  FirewallID     string
  ZKProof        []byte           // Serialized ZKP
  BlockchainHash string           // Tx hash on-chain
  Verified       bool
}

func (logger *IndependenceLogger) ProveEvasion(
  ctx context.Context,
  firewallID string,
  location string,
) (*IndependenceProof, error) {
  // 1. Record event (local)
  timestamp := time.Now().Unix()
  nonce := generateRandomNonce()
  
  commitment := hash(firewallID, timestamp, nonce)
  
  // 2. Generate ZKP (using libsnark or similar)
  zkProof := generateZKP(commitment, []byte{}, nonce)
  
  // 3. Publish to blockchain
  txHash, err := logger.Contract.LogEvasion(
    &bind.TransactOpts{From: logger.AccountAddress},
    commitment,
    location,
    zkProof,
  )
  
  // 4. Wait for confirmation
  receipt := waitForReceipt(ctx, logger.Web3Client, txHash)
  
  return &IndependenceProof{
    ProofID:        generateUUID(),
    Region:         location,
    Timestamp:      timestamp,
    FirewallID:     firewallID,
    ZKProof:        zkProof,
    BlockchainHash: txHash,
    Verified:       receipt.Status == 1,
  }, nil
}

func (logger *IndependenceLogger) GetRegionStats(
  ctx context.Context,
  region string,
  days int,
) (*CensorshipStats, error) {
  // Query blockchain for all CensorshipEvaded events in region
  // Filter by timestamp
  // Aggregate counts
  return &CensorshipStats{
    Region:       region,
    TimeframeDays: days,
    EvictionCount: count,
    Timestamp:     time.Now().Unix(),
  }, nil
}
```

---

### Module 4: Satellite Fallback

#### Failover Logic with Traffic Tiering

**Problem:** LoRa mesh operates at kilobit/s speeds. Routing VDI (screen streaming, MB/s) or Thump workloads (GB-scale migration) through LoRa causes network collapse.

**Solution:** **Strict Traffic Tiering** — LoRa restricted to Control Plane only:

```
Primary Connection Health Monitor:
  ├─ Latency check (every 3 seconds)
  ├─ Packet loss detection (every 10 seconds)
  ├─ Bandwidth measurement (every 30 seconds)
  └─ Reputation score: (latency + loss + bandwidth) → health%

Failover Trigger:
  IF health < 60% OR latency > 500ms THEN
    
    IF still_have_satellite_uplink (Starlink/Kuiper):
      → Route through satellite (maintains data plane)
      → Continue VDI, Thump workloads normally
    
    ELSE IF LoRa_mesh_available:
      → STOP all data plane traffic (VDI, workload migration)
      → LoRa carries ONLY control plane:
         • Thump heartbeat signals (alive/dead status)
         • Emergency control commands (pause, resume, checkpoint)
         • Warren mesh announcements (DHT updates)
      → Hold workload migration requests until ground network recovers
      → Notify user: "Network resilience mode (control only)"
      
    ELSE:
      → Total network loss
      → Local cache serving only (Crovi cached desktop images, etc.)
```

**Bandwidth Guarantee:**
- LoRa throughput: ~50 bps - 50 kbps (depending on range)
- Control plane packets: ~100 bytes/message
- Capacity: ~500 control messages/second (sustainable heartbeat)
- Data plane: Explicitly disabled with DROP rule

#### Node Implementation

```go
type SatelliteNode struct {
  NodeID              string
  StarlingClient      *starlink.Client      // Starlink API
  KuperClient        *kuiper.Client        // Amazon Kuiper API
  LoRaMesh           *lora.MeshController
  
  PrimaryRoute       *Route
  FailoverRoutes     []*Route
  
  HealthMonitor      *HealthMonitor
}

func (sn *SatelliteNode) MonitorPrimaryHealth(ctx context.Context) {
  ticker := time.NewTicker(3 * time.Second)
  defer ticker.Stop()
  
  for range ticker.C {
    latency := measureLatency()
    loss := measurePacketLoss()
    bandwidth := measureBandwidth()
    
    health := (1.0 - loss/100.0) * (1000.0 / (latency + 1)) * (bandwidth / 100.0)
    
    if health < 0.6 || latency > 500 {
      sn.ActivateFailover(ctx)
    }
  }
}

func (sn *SatelliteNode) ActivateFailover(ctx context.Context) error {
  // Try Starlink first (lower latency)
  if sn.tryStarlink(ctx) == nil {
    sn.trafficMode = "FULL_DATA_PLANE"  // All traffic allowed
    return nil
  }
  
  // Fallback to Kuiper
  if sn.tryKuiper(ctx) == nil {
    sn.trafficMode = "FULL_DATA_PLANE"  // All traffic allowed
    return nil
  }
  
  // Last resort: LoRa mesh (CONTROL PLANE ONLY)
  if sn.tryLoRaMesh(ctx) == nil {
    sn.trafficMode = "CONTROL_PLANE_ONLY"  // DROP data plane traffic
    sn.dropDataPlaneTraffic()             // Pause VDI, workload migration
    sn.enableControlHeartbeat()           // Enable heartbeat + control commands
    sn.notifyThump("resilience_mode_activated")  // Notify Thump
    return nil
  }
  
  // All failed
  sn.trafficMode = "LOCAL_CACHE_ONLY"  // Serve from local cache only
  return errors.New("all failover routes exhausted")
}

func (sn *SatelliteNode) dropDataPlaneTraffic() {
  // Explicitly drop VDI streams, workload data
  sn.policyEngine.SetRule("VDI_*", "DROP")
  sn.policyEngine.SetRule("THUMP_WORKLOAD_MIGRATION", "DROP")
  log.Info("Data plane traffic blocked: LoRa control-plane-only mode")
}

func (sn *SatelliteNode) enableControlHeartbeat() {
  // Small, frequent heartbeat packets (~100 bytes every 5 seconds)
  go func() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
      sn.sendHeartbeatViaLoRa()  // Proves node is alive
    }
  }()
}

func (sn *SatelliteNode) tryStarlink(ctx context.Context) error {
  gateway, err := sn.StarlingClient.GetNearestGateway(
    ctx,
    sn.CurrentLocation.Latitude,
    sn.CurrentLocation.Longitude,
  )
  
  if err != nil {
    return err
  }
  
  // Establish connection
  conn, err := sn.StarlingClient.EstablishTunnel(ctx, gateway)
  
  // Update routing table
  sn.PrimaryRoute.Failover = conn
  
  return nil
}
```

#### LoRa Mesh Incentive Model

```protobuf
message LoRaRelayReward {
  string relay_node_id;
  uint64 bytes_relayed;
  uint32 uptime_percent;
  
  // Token reward = base_rate * bytes_relayed * uptime_factor
  float base_rate = 0.00001;  // tokens per byte
  float reward = bytes_relayed * uptime_percent / 100.0 * base_rate;
}

// Settlement every 24 hours on-chain
```

---

## Integration Layer

### Crovi Integration

```
┌─────────────────────────────────────┐
│     Crovi VDI Authentication        │
│     (OIDC, SSO)                     │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│     Warren Policy Injection         │
│     (User → Policy Rules)           │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│    Warren Privacy Gateway           │
│    (Auto-encrypt VDI traffic)       │
└─────────────────────────────────────┘
           ↓
┌─────────────────────────────────────┐
│    Warren ISP Marketplace           │
│    (Route via cheapest path)        │
└─────────────────────────────────────┘
```

### Thump Integration

```
Thump Event: Workload Failover Detected
    ↓
Warren Satellite Fallback: Check connection health
    ↓
IF primary_route_compromised THEN
  - Thump migrates workload
  - Warren re-provisions network path
  - Zero-downtime transition
ELSE
  - Normal failover (no network change)
```

---

## Data Flow

### Complete User Session Flow

```
1. User logs into Crovi (VDI)
   └─ Crovi → Warren: "User XYZ logged in, assign policy set ABC"

2. Warren Privacy Gateway receives VDI desktop traffic
   └─ Classify: "User watching streaming video → policy: entertainment"
   └─ Determine transparency: "intent_only" (hide specific URL)

3. Warren routes through ISP Marketplace
   └─ "Find me the cheapest 10Mbps path for entertainment category"
   └─ Smart contract finds: "Node 5 selling 50Mbps @ $0.40/GB"

4. Packet flow:
   [VDI Traffic] 
      → [Encryption: ChaCha20-Poly1305]
      → [Warren Privacy Header: "entertainment"]
      → [Route: Node5 → ISP Node7 → Public Internet]

5. Independence Logger (optional)
   └─ IF user is in censored region:
      └─ Log "Successfully evaded censorship" to blockchain

6. Satellite Failover (always active)
   └─ Monitor: Node5 → ISP Node7 connection
   └─ IF latency > 500ms:
      └─ Failover to Starlink gateway

7. Thump Integration (background)
   └─ IF workload needs migration:
      └─ Warren re-provisions entire network path to new node
```

---

## Security Model

### Threat Model

| Threat | Attacker | Mitigation |
|--------|----------|-----------|
| **Network eavesdropping** | ISP, government | End-to-end encryption (ChaCha20-Poly1305) |
| **Metadata leakage** | Passive observer | Privacy-first routing + Traffic Obfuscation (masquerade as HTTPS) |
| **DPI protocol detection** | Government (Active probing) | Traffic Obfuscation layer — Warren disguised as standard TLS/HTTPS |
| **Traffic analysis** | Sophisticated adversary | Constant-rate padding, mixing |
| **Wallet address tracking** | Blockchain analyst | Relayer-assisted anonymous proof submission (user wallet never on-chain) |
| **Node compromise** | Malicious ISP marketplace seller | Reputation system, collateral deposits, Off-chain payment channels |
| **Blockchain attack** | 51% attacker | Use Ethereum/Solana (high security assumption) |
| **Denial of Service** | Network-level attacker | Rate limiting per node, Proof-of-Work on queries |
| **LoRa mesh overload** | Attacker flooding mesh | Control-plane-only traffic restriction on LoRa (data plane explicitly dropped) |

### Encryption Standards

- **Transport:** ChaCha20-Poly1305 (default), AES-256-GCM (legacy)
- **Key Exchange:** X25519 (Elliptic Curve Diffie-Hellman)
- **Hashing:** SHA-256 or BLAKE3
- **Random:** Crypto-secure RNG (getrandom on Linux, CNG on Windows)

### Privacy Guarantees

1. **Data Privacy:** No node (not even Warren core developers) can read user data
2. **Metadata Privacy:** 
   - Warren Privacy Gateway ensures ISP sees only policy intent
   - Traffic Obfuscation layer masks Warren protocols as standard HTTPS/TLS (defeats DPI detection)
3. **Financial Privacy:** 
   - Blockchain transactions are pseudonymous (address-based, not name-based)
   - Off-chain Payment Channels eliminate on-chain gas fee overhead (no public ledger of micro-transactions)
4. **Anonymity in Proof Submission:** 
   - Relayer-assisted anonymous proof logging prevents wallet-address tracking
   - User wallet address is never published on blockchain
5. **Resilience Privacy:** 
   - LoRa mesh restricted to control-plane-only (no data exfiltration risk)
   - Location privacy maintained even during failover scenarios
6. **Location Privacy:** LoRa + satellite failover obfuscate geographic location

---

## Implementation Roadmap

### Phase 1: ISP Marketplace MVP (Months 1–3)

**Goals:**
- [ ] Warren Core Protocol (P2P mesh, DHT)
- [ ] Smart contract for bandwidth trading (Ethereum testnet)
- [ ] Marketplace node implementation
- [ ] Tunnel creation (ChaCha20-Poly1305)
- [ ] Basic reputation system

**Deliverables:**
- CLI tool: `warren marketplace list`, `warren marketplace buy`
- Local testnet with 5–10 nodes
- Documentation

**Tech Stack:** Go + Solidity + libp2p

---

### Phase 2: Privacy Gateway + Logger (Months 4–6)

**Goals:**
- [ ] Privacy-first routing protocol (DPN)
- [ ] Policy engine (YAML-based user policies)
- [ ] Independence Logger (ZKP generation)
- [ ] Blockchain logging (Ethereum mainnet)

**Deliverables:**
- CLI: `warren privacy policy set`
- Dashboard: Real-time censorship stats

**Tech Stack:** Go + Ethereum Goerli/Sepolia + libsnark

---

### Phase 3: Satellite Fallback + Integrations (Months 7–9)

**Goals:**
- [ ] Starlink/Kuiper API integration
- [ ] LoRa mesh controller
- [ ] Crovi VDI integration
- [ ] Thump workload relocation sync

**Deliverables:**
- Auto-failover demo (primary → satellite)
- Crovi plugin: auto-apply Warren privacy policies

---

### Phase 4: General Availability + Horizontal Expansion (Months 10–12+)

**Goals:**
- [ ] Production mainnet deployment
- [ ] Warren DNS (domain name resolution over Warren network)
- [ ] Warren Email (SMTP/IMAP over Warren)
- [ ] Warren Storage (object storage over Warren)

---

## References

- [libp2p specification](https://libp2p.io/spec/)
- [Ethereum Yellow Paper](https://ethereum.org/en/developers/docs/)
- [ChaCha20-Poly1305 RFC 8439](https://tools.ietf.org/html/rfc8439)
- [Zero-Knowledge Proof primer](https://en.wikipedia.org/wiki/Zero-knowledge_proof)
