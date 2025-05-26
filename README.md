# tron-tss

A demonstration of multi-party threshold secret sharing (TSS) implementation for the Tron blockchain.

## Overview

This project provides a demonstration of a distributed secret management system for Tron, supporting threshold ECDSA key generation and signing. It is designed to allow multiple parties to collaboratively manage private keys without any single party ever possessing the full key.

## Project Structure

```bash
cmd/
    secret_manager_all_in_one/  # All TSS logic in one file for demo
    secret_manager_coordinator/ # Coordinator node for TSS demo
    secret_manager_party_1-5/   # Party nodes for for TSS demo
config/                         # Configuration files and code
data/                           # Test fixtures and checkpoint data
internal/                       # Internal Go packages (MPC logic, Tron integration, utilities)
types/                          # Type definitions
```

## Demonstration

1. **Start Party Nodes**

In separate terminals, start each party:

```bash
go run ./cmd/secret_manager_party_1
go run ./cmd/secret_manager_party_2
go run ./cmd/secret_manager_party_3
go run ./cmd/secret_manager_party_4
go run ./cmd/secret_manager_party_5
```

2. **Start the Coordinator**

Next, start the coordinator to initiate key generation and signing:

```bash
go run ./cmd/secret_manager_coordinator
```
