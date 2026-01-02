# ENTROPY // X402 SYSTEM TERMINAL

ENTROPY is a brutalist, agentic terminal designed for the orchestration of anonymous, ephemeral cloud infrastructure. It serves as the primary client for the X402 Vending Machine, facilitating zero-knowledge cloud deployments settled via the Base Network L2.

The system is designed for the Agentic Web, allowing both humans and AI agents to discover, negotiate, and settle cloud leases without centralized accounts or forensic retention.

## CORE PRINCIPLES

1. PRIVACY BY PROXY: The system acts as a buffer between the user and upstream hardware providers.
2. EPHEMERALITY: Nodes are transient. Upon lease expiry, instances are purged from the physical realm.
3. PROTOCOL SETTLEMENT: All resource allocations require settlement via the x402 HTTP_402_PAYMENT_REQUIRED protocol.
4. SECURE IDENTITY: Private keys never touch the local database. They are stored exclusively in the host's native secure keyring.

## INSTALLATION

### From Source
Ensure Go 1.22 or higher is installed:
```bash
go install github.com/x402/entropy@latest
```

### From Binary
Download the pre-compiled binary for your architecture from the GitHub Releases page. Move the binary to your system path.

## QUICKSTART

Initialize the terminal by linking your EVM identity:
```bash
entropy login
```
You will be prompted for your private key. This is stored securely in your OS keychain (macOS Keychain, Windows Credential Manager, or Linux Secret Service).

Provision your first node:
```bash
entropy up --tier eco-small --alias dev-node
```

Connect to the instance:
```bash
entropy ssh dev-node
```

## COMMAND REFERENCE

### login
Securely links your EVM wallet. Derives your public address automatically and stores the signer in the system keyring.

### up
Provisions a new VM.
Flags:
- --tier, -t: hardware specification (eco-small, standard, monster)
- --region, -r: geographic location (nbg1, ash, hil, sin)
- --duration, -l: lease length (1h, 24h, 168h)
- --key, -k: path to public SSH key (optional; system will generate one if omitted)
- --alias, -a: local nickname for the instance

### ls
Displays the fleet manifest. Synchronizes local SQLite metadata with remote orchestrator status to show ALIVE vs DEAD nodes and real-time TTL.

### ssh [alias]
Establishes a secure shell connection. Automatically handles identity files and bypasses known_hosts pollution for ephemeral IPs.

### renew [alias]
Extends the lease of an active node. Requires additional x402 settlement.

### rm [alias]
Immediate teardown signal. Destroys the remote instance and wipes the local registry entry.

## THE TUI (INTERACTIVE TERMINAL)

Running entropy without arguments launches the interactive dashboard.

Controls:
- CTRL+R: Synchronize fleet status with orchestrator.
- S: Select node and drop into SSH session.
- R: Renew selected node lease.
- D: Terminate selected node.
- Q: Quit terminal.

## ARCHITECTURE

ENTROPY maintains a local SQLite database at ~/.config/entropy/entropy.db to store metadata, aliases, and SSH key mappings. This allows the tool to remain robust even when the remote orchestrator is under heavy load or offline. 

Private keys are never stored in this database. They are retrieved from the system keyring only at the moment of transaction signing.

## PROTOCOL

Settlement Layer: Base Network (EIP155:8453 / 84532)
Payment Scheme: x402 Exact
Infrastructure Provider: Hetzner Cloud (Proxied)

(c) 2026 X402_INFRASTRUCTURE // DISPOSABLE_CLOUD
