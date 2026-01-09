# ENTROPY // X402 SYSTEM TERMINAL

ENTROPY is a brutalist, agentic terminal designed for the orchestration of anonymous, ephemeral cloud infrastructure. It serves as the primary client for the X402 Vending Machine, facilitating zero-knowledge cloud deployments settled via **Base Network (USDC)** or the **Monero (XMR)** network.

The system is designed for the Agentic Web, allowing both humans and AI agents to discover, negotiate, and settle cloud leases without centralized accounts, forensic retention, or KYC.

## CORE PRINCIPLES

1. **PRIVACY BY PROXY:** The system acts as a buffer between the user and upstream hardware providers.
2. **EPHEMERALITY:** Nodes are transient. Upon lease expiry, instances are purged from the physical realm.
3. **PROTOCOL SETTLEMENT:** All resource allocations require settlement via the x402 HTTP_402_PAYMENT_REQUIRED protocol.
4. **SECURE IDENTITY:** Private keys never touch the local database. They are stored exclusively in the host's native secure keyring.
5. **DETERMINISTIC ANONYMITY:** Identity is derived from your wallet. If you have the keys, you have the account. No "sign-up" required.

## INSTALLATION

### From Source
Ensure Go 1.22 or higher is installed:
```bash
go install github.com/x402-Systems/entropy@latest
```

## QUICKSTART

Initialize the terminal by linking a wallet identity.

**Path A: The Identity Bridge (EVM)**
Links your account to an EVM address. You can pay with USDC or XMR.
```bash
entropy login evm
```

**Path B: The Ghost Path (Monero-Only)**
Maximum privacy. No EVM wallet required. Identity is derived from your XMR wallet.
```bash
entropy login xmr --rpc http://127.0.0.1:18084/json_rpc
```

Provision a node using Monero:
```bash
entropy up --tier eco-small --alias ghost-node --pay xmr
```

## IDENTITY & PRIVACY

ENTROPY uses a **Deterministic Identity** system (`PayerID`). 

- **EVM Users:** Your `PayerID` is your `0x...` address.
- **XMR Users:** Your `PayerID` is a stable hash derived from your primary Monero address.
- **Privacy Shield:** When paying with XMR, the Facilitator sidecar only sees a secondary hash of your `PayerID`, preventing the merchant from linking your on-chain XMR movements to your management identity.

## COMMAND REFERENCE

### login [evm | xmr]
Securely links a wallet. 
- `evm`: Prompts for a private key (stored in keyring).
- `xmr`: Connects to `monero-wallet-rpc` to anchor your identity to your XMR wallet.

### up
Provisions a new VM.
Flags:
- --tier, -t: hardware specification (eco-small, standard, monster)
- --region, -r: geographic location (nbg1, ash, hil, sin)
- --duration, -l: lease length (1h, 24h, 168h)
- --pay, -p: payment method (`usdc` or `xmr`). Defaults to `usdc` if EVM is linked.
- --key, -k: path to public SSH key (optional)
- --alias, -a: local nickname for the instance
- --json: Output raw JSON metadata

### ssh [alias]
Establishes a secure shell connection. Automatically handles identity files and bypasses known_hosts pollution for ephemeral IPs.

### ls
Displays the fleet manifest. Synchronizes local metadata with the remote orchestrator.
**Note:** If paying with XMR, the synchronization requires a verification loop of approximately 30-60 seconds to catch mempool inclusions.

### renew [alias]
Extends the lease of an active node. Supports `--pay xmr`.

### rm [alias]
Immediate teardown signal. Destroys the remote instance. 

### options / stats
Queries the orchestrator for live resource manifests and system telemetry.

## THE TUI (INTERACTIVE TERMINAL)

Running `entropy` without arguments launches the interactive dashboard.

**⚠️ BILLING WARNING:** 
The TUI maintains real-time synchronization with the X402 Orchestrator. 
- **Auto-Sync:** The fleet status refreshes every 20 seconds.
- **Cost:** Each refresh triggers a `$0.001` settlement.
- **XMR Volatility:** XMR quotes are valid for 1 hour. If a payment is not detected within the window, the invoice is purged by the Facilitator Reaper and must be re-negotiated.

Controls:
- **N**: Provision a new node (includes protocol selection toggle).
- **S**: Select node and drop into SSH session.
- **CTRL+R**: Force manual fleet sync.
- **D**: Terminate selected node.
- **Q**: Quit terminal.

## ARCHITECTURE

ENTROPY maintains a local SQLite database at `~/.config/entropy/entropy.db`.
- **Identity Storage:** OS Secure Keyring.
- **Monero Requirement:** Managed via `monero-wallet-rpc`.
- **Facilitator:** Rust-based sidecar for XMR `check_tx_key` verification.

## PROTOCOL

**Settlement Layers:** 
- Base Network (EIP155:8453 / 84532)
- Monero (Mainnet / Stagenet)

**Payment Scheme:** x402 Exact

**Infrastructure Provider:** Hetzner Cloud (Proxied/Disposable)

(c) 2026 X402_INFRASTRUCTURE // DISPOSABLE_CLOUD
