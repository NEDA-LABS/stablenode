## NedaPay Stablenode Aggregator

The Stablenode aggregator simplifies and automates how liquidity flows between various provision nodes and user-created orders, enabling seamless on/off-ramp operations for cryptocurrency payments.


## Protocol Architecture

![image](https://drive.google.com/uc?export=view&id=1UKPMkrSawn_brbE_YQg4S6epqV_fIzh3)

**Create Order**: Users create on/off ramp orders (Payment Intents) on the Gateway Smart Contract (escrow) through the NedaPay Sender API.

**Aggregate**: The Stablenode aggregator indexes orders and assigns them to one or more provision nodes operated by liquidity providers.

**Fulfill**: Provision nodes automatically disburse funds to recipients' local bank accounts or mobile money wallets via connections to payment service providers (PSPs).

---

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

3. **Configure Blockchain Service Provider** (see next section)

4. Run a provision node and connect it to your local aggregator by following the [Provider Setup Guide](PROVIDER_SETUP.md).

That's it! The server will now be running at http://localhost:8000. You can use an API testing tool like Postman or cURL to interact with the Sender API using the sandbox API Key `11f93de0-d304-4498-8b7b-6cecbc5b2dd8`.

---

## Blockchain Service Providers

The aggregator supports two blockchain service providers for smart account management:

### **Option 1: Alchemy (Recommended - Lower Cost)**

Alchemy provides ERC-4337 Account Abstraction infrastructure with significant cost savings over Thirdweb Engine.

**Cost**: $0-49/month (free tier sufficient for most use cases)

#### Setup Steps:

1. **Get Alchemy API Key**
   - Sign up at [Alchemy Dashboard](https://dashboard.alchemy.com/)
   - Create a new app for your target networks (Base, Ethereum, Polygon, etc.)
   - Copy your API key

2. **Deploy Smart Account** (One-time per chain)
   ```bash
   # Set your environment variables first in .env
   # Then deploy using the deployment script
   go run cmd/deploy_smart_account/main.go
   
   # This will output your smart account address
   # Example: 0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
   ```

3. **Configure Environment Variables**
   
   Update your `.env` file:
   ```bash
   # Alchemy Configuration
   ALCHEMY_API_KEY=your_alchemy_api_key_here
   ALCHEMY_BASE_URL=https://api.g.alchemy.com/v2
   ALCHEMY_GAS_POLICY_ID=your_gas_policy_id  # Optional
   
   # Service Selection
   USE_ALCHEMY_SERVICE=true
   USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
   
   # Smart Account Configuration
   SMART_ACCOUNT_OWNER_ADDRESS=0xYourOwnerAddress
   SMART_ACCOUNT_OWNER_PRIVATE_KEY=0xYourPrivateKey
   
   # Deployed Smart Account (from step 2)
   AGGREGATOR_SMART_ACCOUNT=0x8493c7FF99dedD3da3eaCDC56ff474c12Ac3e67D
   ```

4. **Verify Setup**
   ```bash
   # Make verification script executable
   chmod +x verify_alchemy.sh
   
   # Run verification
   sudo ./verify_alchemy.sh
   ```
   
   You should see:
   ```
   ✅ USE_ALCHEMY_SERVICE=true
   ✅ USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
   ✅ ALCHEMY_API_KEY configured
   ✅ SMART_ACCOUNT_OWNER_ADDRESS configured
   ```

**Important Notes:**
- Smart accounts are **chain-specific** - you need to deploy on each chain you support
- You can deploy to the **same address** on multiple chains using the same owner + salt
- For multi-chain operations, see [Multi-Chain Deployment Guide](ALCHEMY_SETUP.md)

---

### **Option 2: Thirdweb Engine (Legacy)**

Thirdweb Engine provides managed wallet infrastructure with higher costs.

**Cost**: $99-999/month subscription

#### Setup Steps:

1. **Get Thirdweb Credentials**
   - Deploy Thirdweb Engine or use hosted version
   - Get your Engine URL, access token, and secret key

2. **Configure Environment Variables**
   
   Update your `.env` file:
   ```bash
   # Thirdweb Engine Configuration
   ENGINE_BASE_URL=https://your-engine.com
   ENGINE_ACCESS_TOKEN=your-vault-token
   THIRDWEB_SECRET_KEY=your-thirdweb-key
   
   # Service Selection
   USE_ALCHEMY_SERVICE=false
   USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=false
   
   # Aggregator Account (managed by Thirdweb)
   AGGREGATOR_SMART_ACCOUNT=0xYourThirdwebAccount
   AGGREGATOR_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----..."
   ```

---

### **Hybrid Approach (Recommended for Migration)**

You can use Alchemy for receive addresses (cost savings) while keeping Thirdweb for operational accounts:

```bash
# Use Thirdweb for operational account
USE_ALCHEMY_SERVICE=false
AGGREGATOR_SMART_ACCOUNT=0xYourExistingThirdwebAccount

# Use Alchemy for receive addresses (saves cost)
USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true
SMART_ACCOUNT_OWNER_ADDRESS=0xYourOwnerAddress
```

This provides immediate cost savings without requiring full migration.

---

### **Testing Your Configuration**

After configuration, test that everything works:

#### **Quick Test (Without Docker)**

Test Alchemy smart account creation directly without rebuilding Docker:

```bash
# Run the standalone test
go run cmd/test_alchemy/main.go

# Expected output:
# 🧪 Testing Alchemy Smart Account Creation
# ==========================================
# Chain ID: 84532
# Owner Address: 0xFb84E5503bD20526f2579193411Dd0993d080775
# Network: base-sepolia
# RPC Endpoint: https://base-sepolia.g.alchemy.com/v2
# 
# 📝 Creating smart account...
# ✅ Success!
# Smart Account Address: 0x633CA25f8CD81D2b5D14A18346bEb8cc54b179c8
```

**Run multiple times** to verify unique addresses are generated for each receive address.

#### **Full Integration Test**

```bash
# 1. Start the services
docker-compose up --build -d

# 2. Check logs for service initialization
docker logs nedapay_aggregator --tail 50

# 3. Look for these messages:
#    "Initializing Service Manager"
#    "Using Alchemy service: true" (or false if using Thirdweb)

# 4. Verify account before creating order
curl -X POST http://localhost:8000/v1/verify-account \
  -H "Content-Type: application/json" \
  -d '{
    "institution": "ABNGNGLA",
    "accountIdentifier": "0123456789"
  }'

# 5. Create a test order via API
curl -X POST http://localhost:8000/v1/sender/orders \
  -H "Content-Type: application/json" \
  -H "X-API-Key: 11f93de0-d304-4498-8b7b-6cecbc5b2dd8" \
  -d '{
    "amount": "100",
    "token": "DAI",
    "chain": "base-sepolia"
  }'

# 5. Check logs for receive address creation
docker logs nedapay_aggregator 2>&1 | grep -i "receive address"
# Should see: "Creating receive address via Alchemy" or "via Thirdweb"
```

---

### **Migration Guide**

If you're migrating from Thirdweb to Alchemy:

1. Read the [Backend Migration Guide](BACKEND_MIGRATION_GUIDE.md)
2. Follow the [Alchemy Migration Strategy](ALCHEMY_MIGRATION_STRATEGY.md)
3. Check [Alchemy Setup Details](ALCHEMY_SETUP.md) for advanced configuration

**Quick Migration Checklist:**
- [ ] Create Alchemy account and get API key
- [ ] Deploy smart account on testnet
- [ ] Update .env with Alchemy configuration
- [ ] Enable hybrid mode (`USE_ALCHEMY_FOR_RECEIVE_ADDRESSES=true`)
- [ ] Test receive address generation
- [ ] Monitor logs and verify functionality
- [ ] Deploy to production
- [ ] (Optional) Deploy operational accounts on other chains
- [ ] Cancel Thirdweb subscription

---

### **Cost Comparison**

| Feature | Thirdweb Engine | Alchemy |
|---------|----------------|---------|
| **Monthly Subscription** | $99-999 | $0-49 (free tier sufficient) |
| **Smart Account Creation** | Managed by Engine | Self-deployed (one-time) |
| **Key Management** | Managed vault | Self-managed |
| **Gas Fees** | Same | Same |
| **Advanced Features** | Included | Included |
| **Best For** | Managed infrastructure | Cost-conscious teams |

**Estimated Monthly Savings**: $99-950 by switching to Alchemy

---

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
