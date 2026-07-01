# Warren Technical Architecture & Design

> **Revision note (this update):** Reframes the censorship-evasion strategy around
> **node-scale diversity** (the ISP Marketplace's pool of ordinary residential nodes)
> as the primary anti-blocklisting defense, replaces naive SNI-spoofing with a
> REALITY-style TLS borrowing scheme for Module 2, and splits Module 3 into a
> local-only authentication proof (no on-chain trace) and a separate opt-in
> aggregate logging path (kept to preserve the Independence Logger revenue model).
> Rationale for each change is documented inline.

## Table of Contents

1. [Core Protocol](#core-protocol)
2. [Anti-Blocklisting Defense Model](#anti-blocklisting-defense-model)
3. [Module Specifications](#module-specifications)
4. [Integration Layer](#integration-layer)
5. [Data Flow](#data-flow)
6. [Security Model](#security-model)
7. [Implementation Roadmap](#implementation-roadmap)

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
| **Micro-payments** | **Off-chain Payment Channels**, settled on a hybrid trigger: every 10MB transferred **or** every 5 seconds elapsed, whichever comes first (eliminates gas fee overhead without under-specifying the settlement cadence) |
| **Protocol Camouflage** | REALITY-style TLS session borrowing (see [Module 2](#module-2-privacy-gateway-dpn)) — replaces naive SNI spoofing |
| **Node Discovery** | Multi-channel bootstrap (DHT + out-of-band bridge distribution) — no single broker that can become the first block target |

### Node Types

1. **Routing Node** (runs all the time)
   - Stores DHT entries
   - Relays traffic
   - Announces available capacity

2. **Marketplace Node** (sells bandwidth)
   - Advertising node + routing
   - Tracks bandwidth utilization
   - Publishes pricing
   - **Doubles as a camouflage relay** — ordinary residential IP, short-lived, part of the anti-blocklisting node pool (see next section)

3. **Privacy Node** (runs policy engine)
   - Encryption/decryption
   - Policy decision-making
   - Never stores plaintext user data
   - Performs REALITY handshake borrowing on behalf of connecting clients

4. **Logger Node** (records independence proofs)
   - ZKP verification (both local-passport and aggregate-evasion proofs)
   - Blockchain interaction (aggregate proofs only — see [Module 3](#module-3-independence-verification--logger))
   - Analytics aggregation

5. **Satellite Node** (failover gateway)
   - Starlink/Kuiper uplink
   - LoRa mesh coordinator
   - Automatic routing failover

---

## Anti-Blocklisting Defense Model

### Why protocol camouflage alone is not enough

State-level censors (China's GFW and similar systems) do not primarily rely on decoding
protocols — they block by **IP/ASN reputation** and by **active probing** (connecting back
to a suspected server to check whether it behaves like the site it claims to be). This is
documented history: domain fronting (relying on a CDN's SNI/Host mismatch) is dead because
major CDNs (Google, Cloudflare, AWS) disabled it; commercial VPN providers are blocked within
days of launch because their exit servers sit on a small, enumerable set of datacenter IP
ranges, regardless of how well the tunnel protocol is disguised.

**Consequence for Warren:** protocol-level disguise (Module 2) is necessary but not
sufficient. The durable defense is **making the set of endpoints too large, too ordinary,
and too short-lived to blocklist** — which is exactly what the ISP Marketplace's model of
ordinary residential nodes selling spare bandwidth already provides, if the routing and
discovery layers are designed to exploit it.

### The two-layer model

```
Layer 1 (structural):  ISP Marketplace node pool
                        → thousands of ordinary residential IPs, constantly
                          rotating as sellers join/leave the market
                        → blocking cost for the censor scales with the number
                          of households, not the number of "VPN companies"

Layer 2 (protocol):    Privacy Gateway camouflage
                        → each individual connection, to a passive AND active
                          observer, looks like a real visit to a real site
                        → defeats DPI signature matching + active probing
```

This mirrors the precedent set by Psiphon/Snowflake (structural defense via large,
ephemeral, volunteer-run proxy pools) combined with the REALITY protocol (state-of-the-art
protocol camouflage that survives active probing) — rather than inventing a new approach
from scratch, Warren composes two independently-proven patterns on top of infrastructure
(the Marketplace) it already needed for its bandwidth-trading business.

### Design implication

Module 1 (ISP Marketplace) is no longer just a revenue mechanism — it is reclassified as
**primary infrastructure for censorship resistance**, with Module 2 (Privacy Gateway) as the
per-connection camouflage layer riding on top of it. This changes one thing operationally:
marketplace node selection for a censorship-evasion session should prefer **node diversity
and churn** (many small residential sellers) over **node quality** (few large, stable,
high-bandwidth sellers) — the opposite of what a pure bandwidth-marketplace optimizer would
pick. The routing algorithm needs a `mode=resilience` weighting that trades some throughput
for endpoint diversity.

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

**Forward-looking risk:** full VPN bans (as opposed to VPN-protocol detection) are moving
toward **application whitelisting** — permitting only a fixed set of domestic apps (e.g.
WeChat-class traffic) rather than trying to detect and block every foreign protocol. If a
region moves to this model, "looking like generic HTTPS" stops being sufficient, and the
camouflage target needs to shift toward mimicking an explicitly whitelisted app's traffic
pattern rather than a generic popular website. This is out of scope for the current
implementation phase but should inform which sites are chosen as REALITY targets per region.

#### Protocol Camouflage: REALITY-Style TLS Borrowing (Critical for Censorship Evasion)

**Problem:** If Warren packets have recognizable headers/signatures, government DPI systems
will blacklist them immediately (by IP/ASN blocking). Worse, **naive SNI spoofing does not
work**: if a client's TLS ClientHello claims `SNI = www.google.com` while the destination IP
is a Warren node (not a Google-owned IP/ASN), a state-level DPI system that correlates
SNI-to-IP ownership flags this immediately. Domain fronting (routing through a real CDN edge
that hosts the spoofed name) is the correct fix for this specific failure mode, but is
largely closed off — major CDNs disabled cross-domain fronting after it was weaponized for
circumvention.

**Solution:** **REALITY-style TLS session borrowing**, not SNI spoofing:

- Each Privacy Node is paired with a real, currently-popular HTTPS site it can legitimately
  proxy a TLS handshake through (the actual target of the handshake is real — the node does
  not fabricate a certificate or claim an identity it can't back up)
- A Warren client's traffic is authenticated via a short cryptographic tag embedded in the
  TLS ClientHello's otherwise-unused extension bytes, verifiable only by the target Privacy
  Node's private key. To anyone else — including active probes — the connection is
  indistinguishable from a real visit to the real site, because for non-Warren clients it
  genuinely **is** a real visit (the Privacy Node relays to the real site by default,
  identical to how xray-core's REALITY protocol operates)
- **Active probing resistance:** if a censor's probe connects and does not present the
  correct authentication tag, the Privacy Node transparently proxies to the real site with a
  real, valid certificate chain — the probe sees a legitimate site and has nothing to flag
- This is a different failure mode fix than plain protocol mimicry: mimicry can be detected
  by an adversary who tries the handshake themselves and notices the "site" doesn't behave
  like the real one (session resumption gaps, ALPN mismatches, etc.); TLS borrowing avoids
  this because the fallback path *is* the real site

**Decentralized bootstrap / discovery:** node addresses and the site-pairing list must not be
served from a single central endpoint — that endpoint becomes the first thing blocked. Follow
the Tor-bridge precedent: distribute via multiple independent, low-volume out-of-band
channels (e.g., rotating subsets shared via email autoresponders, encrypted messaging
channels) in addition to DHT discovery, so no single choke point exists.

**Traffic shape normalization:** small random padding (a few dozen bytes) is not sufficient
against ML-based flow classifiers, which fingerprint packet-size distributions and inter-
arrival timing rather than payload signatures — this is a well-documented weakness in
website-fingerprinting research even against Tor. Warren instead normalizes VDI/data-plane
traffic into a small number of fixed packet-size buckets and injects constant-rate cover
traffic during idle periods, so the observable bandwidth profile stays flat regardless of
underlying activity.

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

### Module 3: Independence Verification & Logger

**Design note:** this module is split into two independent proofs with different purposes.
An earlier draft proposed replacing all on-chain evasion logging with a purely local
authentication proof, on the grounds that logging evasion metadata anywhere is a privacy
mistake. That's correct for *authentication*, but conflating it with *logging* would remove
the aggregate censorship statistics that the Independence Logger revenue line (NGO/research
data subscriptions, $20K–100K/year per the business plan) depends on. The two are kept
separate below so neither goal compromises the other.

#### 3a. ZK-Passport: Local Handshake Authorization (no on-chain trace)

Used at P2P handshake time to prove "this is a node holding a valid independence passport"
without revealing which node, and without ever touching the blockchain. Verified peer-to-peer
between the two handshaking nodes only.

**Unlinkability requirement:** a static commitment (e.g. `hash(secret, hardware_serial)`)
reused across sessions is itself a fingerprint — repeated presentation of the same commitment
across different relays/times lets a correlating observer link sessions to the same device,
defeating the purpose. Each handshake must instead present a **fresh nullifier** derived from
the same underlying secret (Semaphore-style), so two sessions from the same device are
provably from *a* valid passport-holder but not provably from the *same* one.

```
Circuit inputs:
  private: passport_secret, hardware_binding
  public:  passport_commitment (registered once, off-chain, with the issuing node)
           session_nullifier = hash(passport_secret, session_epoch)

  Proves:  commitment = hash(passport_secret, hardware_binding)   [knowledge of a valid passport]
       AND session_nullifier correctly derived from passport_secret for this session_epoch
       WITHOUT revealing passport_secret or hardware_binding

Result exchanged over the wire: only (session_nullifier, proof) — never the commitment,
never a wallet address, never touches the blockchain.
```

#### 3b. Aggregate Evasion Logger (on-chain, opt-in, statistics only)

This is the existing Independence Logger design below, **unchanged** — it remains the
mechanism that produces the region-level censorship statistics sold to NGOs/researchers.
Two things distinguish it from 3a and keep it privacy-safe:

- It is **opt-in** and produces only aggregate/batched counts, not per-session records
  correlatable to a specific user's browsing
- It uses the same relayer-assisted anonymity model already designed below, so individual
  wallet addresses never appear on-chain even for this aggregate reporting

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
| **Metadata leakage** | Passive observer | Privacy-first routing + Protocol Camouflage (REALITY-style TLS borrowing) |
| **SNI-to-IP correlation** | Government (Passive + list-based) | TLS borrowing routes to the *real* site's actual infrastructure, not a spoofed SNI on Warren-owned IPs |
| **Active probing** | Government (connects back to suspected server) | Non-authenticated connections fall through to a genuine proxied response from the real site — nothing to flag |
| **IP/ASN blocklisting of exit nodes** | Government (bulk block by network range) | Node-scale defense — ISP Marketplace's large, churning pool of residential nodes, not a small set of datacenter exits |
| **Traffic analysis (ML flow classification)** | Sophisticated adversary | Fixed-bucket packet sizing + constant-rate cover traffic (not just random padding) |
| **Session linkability across handshakes** | Passive/active correlator | ZK-Passport session nullifiers (Module 3a) — no static commitment reused across sessions |
| **Wallet address tracking** | Blockchain analyst | Relayer-assisted anonymous proof submission (user wallet never on-chain); Module 3a proofs never touch chain at all |
| **Node compromise** | Malicious ISP marketplace seller | Reputation system, collateral deposits, Off-chain payment channels |
| **Blockchain attack** | 51% attacker | Use Ethereum/Solana (high security assumption) |
| **Denial of Service** | Network-level attacker | Rate limiting per node, Proof-of-Work on queries |
| **LoRa mesh overload** | Attacker flooding mesh | Control-plane-only traffic restriction on LoRa (data plane explicitly dropped) |
| **Discovery/bootstrap takedown** | Government blocks the node-list source | Multi-channel decentralized bootstrap (DHT + out-of-band bridge distribution), no single broker |

### Encryption Standards

- **Transport:** ChaCha20-Poly1305 (default), AES-256-GCM (legacy)
- **Key Exchange:** X25519 (Elliptic Curve Diffie-Hellman)
- **Hashing:** SHA-256 or BLAKE3
- **Random:** Crypto-secure RNG (getrandom on Linux, CNG on Windows)

### Privacy Guarantees

1. **Data Privacy:** No node (not even Warren core developers) can read user data
2. **Metadata Privacy:** 
   - Warren Privacy Gateway ensures ISP sees only policy intent
   - REALITY-style TLS borrowing masks Warren connections as real visits to real sites,
     surviving both passive DPI signature matching and active probing
3. **Financial Privacy:** 
   - Blockchain transactions are pseudonymous (address-based, not name-based)
   - Off-chain Payment Channels eliminate on-chain gas fee overhead (no public ledger of micro-transactions)
4. **Anonymity in Proof Submission:** 
   - Module 3a (ZK-Passport handshake auth) never touches the blockchain and uses
     per-session nullifiers, so device identity cannot be linked across sessions
   - Module 3b (aggregate evasion logging) uses relayer-assisted anonymous submission —
     user wallet address is never published on blockchain, and only aggregate counts are logged
5. **Resilience Privacy:** 
   - LoRa mesh restricted to control-plane-only (no data exfiltration risk)
   - Location privacy maintained even during failover scenarios
6. **Location Privacy:** LoRa + satellite failover obfuscate geographic location
7. **Anti-Blocklisting (structural, not just cryptographic):**
   - Exit/relay diversity comes from the ISP Marketplace's residential node pool, not a
     small enumerable set of infrastructure IPs — see [Anti-Blocklisting Defense Model](#anti-blocklisting-defense-model)

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
- [ ] REALITY-style TLS borrowing (Module 2 protocol camouflage)
- [ ] Decentralized bootstrap/discovery (multi-channel, no single broker)
- [ ] ZK-Passport local handshake auth with session nullifiers (Module 3a)
- [ ] Independence Logger aggregate ZKP generation (Module 3b)
- [ ] Blockchain logging (Ethereum mainnet) — Module 3b only, aggregate stats

**Deliverables:**
- CLI: `warren privacy policy set`
- Active-probing resistance test harness (isolated DPI simulator, validates against real
  active-probe behavior, not just passive signature-matching tools like nDPI)
- Dashboard: Real-time censorship stats

**Tech Stack:** Go + Ethereum Goerli/Sepolia + libsnark + Circom/Groth16 (Module 3a)

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
