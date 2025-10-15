# Aggregator Smart Account Generation Guide

This guide explains how to set up your own aggregator smart account configuration when forking this repository.

## What These Configurations Are For

### `AGGREGATOR_SMART_ACCOUNT`
- **Purpose**: The Ethereum address of your aggregator's smart contract account (ERC-4337 Account Abstraction wallet)
- **Usage**: This is the on-chain identity of your aggregator that can execute transactions, hold funds, and interact with smart contracts
- **Example**: `0x03Ff9504c7067980c1637BF9400E7b7e3655782c`

### `AGGREGATOR_PUBLIC_KEY` & `AGGREGATOR_PRIVATE_KEY`
- **Purpose**: The cryptographic key pair that controls the aggregator's smart account
- **Usage**: 
  - Private key signs transactions and operations
  - Public key is used for verification and deriving addresses
- **Security**: The private key must be kept secret as it controls the aggregator's funds and operations

## How to Get Your Own

### Option 1: Generate New Keys (Recommended)

1. **Generate a new key pair**:
   ```bash
   # Using OpenSSL to generate a new private key
   openssl ecparam -genkey -name secp256k1 -noout -out private_key.pem
   
   # Or use a tool like MetaMask, hardware wallet, or crypto libraries
   ```

2. **Deploy a Smart Account**:
   - Use the SimpleAccountFactory contract (already in your contracts)
   - Deploy a new smart account using your new public key as the owner
   - This will give you the `AGGREGATOR_SMART_ACCOUNT` address

### Option 2: Use Existing Wallet

1. **Export from existing wallet** (MetaMask, etc.):
   - Export your private key from MetaMask or another wallet
   - Use the corresponding address as your smart account owner

2. **Deploy Smart Account**:
   - Call the `createAccount` function on SimpleAccountFactory
   - Pass your wallet address as the owner parameter

## Step-by-Step Setup

1. **Create your `.env` file**:
   ```bash
   cp .env.example .env
   ```

2. **Generate or import your keys**:
   ```env
   AGGREGATOR_PRIVATE_KEY="your_private_key_here"
   AGGREGATOR_PUBLIC_KEY="your_public_key_here"  
   ```

3. **Deploy Smart Account** (using the existing contracts):
   ```go
   // Use the SimpleAccountFactory contract to create your account
   // This will give you the AGGREGATOR_SMART_ACCOUNT address
   ```

4. **Update your `.env`**:
   ```env
   AGGREGATOR_SMART_ACCOUNT=0xYourNewSmartAccountAddress
   ```

## Important Security Notes

- **Never commit real private keys** to version control
- **Use different keys for different environments** (dev, staging, prod)
- **Consider using hardware wallets** or secure key management for production
- **Fund your smart account** with ETH for gas fees on your target networks

## For Development

You can use the existing example values for local development, but **always generate new keys for any public deployment** to ensure security and avoid conflicts with the original aggregator.

## Smart Account Deployment Process

The smart account deployment typically involves:

1. **Deploy EntryPoint Contract** (if not already deployed on your target network)
2. **Deploy SimpleAccountFactory Contract**
3. **Call `createAccount(owner, salt)`** on the factory
4. **Record the returned address** as your `AGGREGATOR_SMART_ACCOUNT`

## Environment Variables Summary

```env
# Crypto Configuration
AGGREGATOR_PRIVATE_KEY="your_private_key_here"
AGGREGATOR_PUBLIC_KEY="your_public_key_here"
AGGREGATOR_SMART_ACCOUNT="0xYourSmartAccountAddress"

# Other required configs
HD_WALLET_MNEMONIC="your twelve word mnemonic phrase here"
```

## Testing Your Setup

After configuration, test your setup by:

1. Starting the aggregator service
2. Checking that it can connect to your smart account
3. Verifying transaction signing capabilities
4. Testing basic operations like balance queries

Remember to keep your private keys secure and never share them publicly!
