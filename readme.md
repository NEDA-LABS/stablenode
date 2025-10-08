## NedaPay Stablenode Aggregator

The Stablenode aggregator simplifies and automates how liquidity flows between various provision nodes and user-created orders, enabling seamless on/off-ramp operations for cryptocurrency payments.


## Protocol Architecture

![image](https://github.com/user-attachments/assets/fdea36e5-9f54-4b17-bf0d-44d33d96fc62)

**Create Order**: Users create on/off ramp orders (Payment Intents) on the Gateway Smart Contract (escrow) through the NedaPay Sender API.

**Aggregate**: The Stablenode aggregator indexes orders and assigns them to one or more provision nodes operated by liquidity providers.

**Fulfill**: Provision nodes automatically disburse funds to recipients' local bank accounts or mobile money wallets via connections to payment service providers (PSPs).

## Development Setup

Pre-requisite: Install required dependencies:
- [Docker Compose](https://docs.docker.com/compose/install/)
- [Ent](https://entgo.io/docs/getting-started/) for database ORM
- [Atlas](https://atlasgo.io/guides/evaluation/install#install-atlas-locally) for database migrations

To set up your development environment, follow these steps:

1. Setup the Stablenode aggregator repo on your local machine.

```bash
# clone the repo
git clone https://github.com/NEDA-LABS/stablenode.git

cd stablenode

# copy environment variables
cp .env.example .env
```

2. Start and seed the development environment:
```bash

# build the image
docker-compose build

# run containers
docker-compose up -d

# make script executable
chmod +x scripts/import_db.sh

# run the script to seed db with sample configured sender & provider profile
./scripts/import_db.sh -h localhost
```

3. Run a provision node and connect it to your local aggregator by following the [Provider Setup Guide](PROVIDER_SETUP.md).

That's it! The server will now be running at http://localhost:8000. You can use an API testing tool like Postman or cURL to interact with the Sender API using the sandbox API Key `11f93de0-d304-4498-8b7b-6cecbc5b2dd8`.


## Usage
- Interact with the Sender API using the sandbox API Key `11f93de0-d304-4498-8b7b-6cecbc5b2dd8`
- Payment orders initiated using the Sender API in sandbox should use the following testnet tokens from the public faucets of their respective networks:
  - **DAI** on Base Sepolia
  - **USDT** on Ethereum Sepolia and Arbitrum Sepolia


## Contributing

We welcome contributions to NedaPay Stablenode! To get started:

1. Fork the repository
2. Create a feature branch
3. Make your changes with appropriate tests
4. Submit a pull request

Our team will review your pull request and work with you to get it merged into the main branch.

If you encounter any issues or have questions, feel free to open an issue on the repository.


## Testing

We use a combination of unit tests and integration tests to ensure the reliability of the codebase.

To run the tests, run the following command:

```bash
# install and run ganache local blockchain
npm install ganache --global
HD_WALLET_MNEMONIC="media nerve fog identify typical physical aspect doll bar fossil frost because"; ganache -m "$HD_WALLET_MNEMONIC" --chain.chainId 1337 -l 21000000

# run all tests
go test ./...

# run a specific test
go test ./path/to/test/file
```
It is mandatory that you write tests for any new features or changes you make to the codebase. Only PRs that include passing tests will be accepted.

## License

[Affero General Public License v3.0](https://choosealicense.com/licenses/agpl-3.0/)