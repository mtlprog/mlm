# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MLM (Montelibero Multi-Level Marketing) is a Go application that distributes EURMTL rewards to Stellar network accounts based on MTLAP token holdings and recommendation relationships. It consists of two binaries:

- **mlm**: Long-running Telegram bot that handles transaction submission via callback buttons
- **mlmc**: CLI tool for generating distribution reports and optionally submitting transactions

## Build & Development Commands

```bash
make build        # Generate sqlc code (run after changing queries.sql)
make run          # Build and run the mlm bot
make test         # Run all tests with verbose output
```

### Database Migrations

Uses goose with PostgreSQL:

```bash
make migrate-up                    # Apply pending migrations
make migrate-status                # Check migration status
make migrate-generate name=foo    # Create new migration
```

Migrations are embedded via `//go:embed migrations/*.sql` and auto-run on mlm startup.

## Architecture

### Core Flow

1. `stellar.Client.Recommenders()` fetches all MTLAP holders from Stellar Horizon API
2. Accounts with `RecommendToMTLA*` data entries become recommenders (min 4 MTLAP required)
3. `distributor.Distributor.CalculateParts()` computes reward distribution based on MTLAP changes since last report
4. 1/3 of the distribution address's EURMTL balance is distributed
5. Reports are stored in PostgreSQL; XDR transactions can be submitted via Telegram bot or CLI

### Key Interfaces (mlm.go)

- `StellarAgregator`: Stellar API operations (balance, recommenders, account details)
- `HorizonClient`: Stellar Horizon client interface
- `Distributor`: Main distribution logic

### Package Structure

- `stellar/`: Horizon client wrapper, MTLAP/EURMTL constants, recommendation parsing
- `distributor/`: Distribution calculation and report creation
- `tgbot/`: Telegram bot for XDR submission callbacks
- `report/`: HTML report formatting for Telegram
- `db/`: sqlc-generated PostgreSQL queries
- `config/`: Environment variable configuration

## Configuration

Environment variables (loaded from `.env`):

- `POSTGRES_DSN`: PostgreSQL connection string
- `TELEGRAM_TOKEN`: Bot token
- `STELLAR_ADDRESS`: Distribution source address
- `STELLAR_SEED`: Signing key for transactions
- `ALLOWED_USER_IDS`: Comma-separated Telegram user IDs for admin access
- `REPORT_TO_CHAT_ID`, `REPORT_TO_MESSAGE_THREAD_ID`: Where to send reports
- `SUBMIT`: Set to "true" to auto-submit transactions
- `WITHOUT_REPORT`: Set to "true" to skip database report creation

## Database

Uses sqlc for type-safe queries. After modifying `queries.sql`, run `make build` to regenerate `db/` package.

Schema in `migrations/`:
- `reports`: Distribution reports with XDR and hash
- `report_recommends`: Recommender-recommended relationships per report
- `report_distributes`: Actual payment amounts per recommender
- `report_conflicts`: When multiple recommenders claim same account
- `states`: Bot state storage
