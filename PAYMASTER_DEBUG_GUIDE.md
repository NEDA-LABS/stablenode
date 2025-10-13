# Paymaster AA23 Error Debug Guide

## What We Added

Enhanced debugging in `services/alchemy.go` to help identify why paymaster requests fail with AA23 errors.

### New Debug Logs

1. **Input Validation** (line 1268-1285)
   - Logs the raw userOp received
   - Lists all keys present in userOp
   - Validates all required fields exist
   - Fails early if any field is missing

2. **Detailed Request Logging** (line 1315-1334)
   - Logs all critical userOp fields
   - Pretty-prints the v07UserOp structure
   - Shows lengths of callData and factoryData
   - Logs the complete JSON request

3. **Enhanced Error Analysis** (line 1367-1424)
   - Extracts error code, message, and data
   - Shows which userOp fields were sent
   - Provides AA23-specific troubleshooting hints

## How to Debug

### Step 1: Run Your Code and Check Logs

Look for these log entries:

```
[DEBUG] getPaymasterData called with userOp
[DEBUG] Requesting paymaster data from Alchemy
[DEBUG] Full v07UserOp:
[DEBUG] Full paymaster request:
```

### Step 2: Verify Required Fields

The logs will show if any required fields are missing:
- `sender` - Smart account address
- `nonce` - Account nonce (0x0 for new accounts)
- `callData` - The transaction data
- `callGasLimit` - Gas for the call
- `verificationGasLimit` - Gas for verification
- `preVerificationGas` - Pre-verification gas
- `maxFeePerGas` - Max gas price
- `maxPriorityFeePerGas` - Priority fee

### Step 3: Check AA23 Error Details

When AA23 occurs, you'll see:

```
[DEBUG] Paymaster request returned error - AA23 means validation/creation failed
[DEBUG] AA23 Error Analysis:
  - Check if smart account exists (if nonce > 0, it should exist)
  - Check if factory/factoryData is correct (if nonce = 0)
  - Check if owner address in factoryData is correct
  - Check if callData is properly encoded
  - Check if gas limits are sufficient
```

## Common AA23 Causes

### 1. Incomplete CallData
**Your error showed:**
```json
"callData":"0x18dfb3c70000000000000000000000000000000000000000000000000000000000000002000000000000000000000000036cbd53842c5426634e7929541ec"
```

This looks truncated. A complete `executeBatch` callData should be much longer.

**Fix:** Check where callData is generated. It should include:
- Function selector: `0x18dfb3c7` (executeBatch)
- Complete ABI-encoded parameters

### 2. Wrong Dummy Signature Format
**Your error showed:**
```json
"dummySignature":"0xfffffffffffffffffffffffffffffff0000000000000000000000000000000007aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1c"
```

**Should be:**
```json
"dummySignature":"0x00fffffffffffffffffffffffffffffff0000000000000000000000000000000007aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1c"
```

Note the `00` prefix (EOA signature type for Light Account v2).

**Status:** ✅ Fixed in code (line 1303)

### 3. Missing UserOp Fields
**Your error showed only:**
```json
{
  "callData": "0x..."
}
```

**Should include:**
```json
{
  "sender": "0x...",
  "nonce": "0x0",
  "factory": "0x...",
  "factoryData": "0x...",
  "callData": "0x...",
  "callGasLimit": "0x...",
  "verificationGasLimit": "0x...",
  "preVerificationGas": "0x...",
  "maxFeePerGas": "0x...",
  "maxPriorityFeePerGas": "0x..."
}
```

**Fix:** Check the code that calls `getPaymasterData()` - it must pass a complete userOp.

### 4. Invalid Factory/FactoryData
If nonce is `0x0` (new account), factory and factoryData must be correct:

**Factory:** `0x0000000000400CdFef5E2714E63d8040b700BC24` (Light Account Factory v2)

**FactoryData format:**
```
0x5fbfb9cf  // createAccount(address,uint256) selector
+ 000000000000000000000000{ownerAddress}  // 32 bytes padded owner
+ {salt}  // 32 bytes salt
```

### 5. Account Already Exists
If nonce > 0, the account should already exist on-chain. If it doesn't, AA23 will occur.

**Check:** Query the blockchain to see if the smart account exists at the sender address.

## Next Steps

### 1. Find Where UserOp is Built

Search your codebase for where the userOp is constructed before calling `getPaymasterData()`:

```bash
cd /home/commendatore/Desktop/NEDA/rails/aggregator
grep -r "getPaymasterData" --include="*.go"
```

### 2. Check the Calling Code

Look at the function that calls `getPaymasterData()` and verify:
- All required fields are set
- CallData is complete (not truncated)
- Gas limits are reasonable values
- Sender address is correct

### 3. Test with Known Good Values

Try with these test values:
```go
userOp := map[string]interface{}{
    "sender": "0xYourSmartAccountAddress",
    "nonce": "0x0",
    "initCode": "0x0000000000400CdFef5E2714E63d8040b700BC245fbfb9cf000000000000000000000000{ownerAddress}{salt}",
    "callData": "0x...", // Complete callData
    "callGasLimit": "0x30000",
    "verificationGasLimit": "0x30000", 
    "preVerificationGas": "0x10000",
    "maxFeePerGas": "0x3b9aca00",
    "maxPriorityFeePerGas": "0x3b9aca00",
    "paymasterAndData": "0x",
    "signature": "0x",
}
```

## Reading the Debug Logs

### Example Good Log Output:
```
[DEBUG] getPaymasterData called with userOp
  ChainID: 84532
  UserOpKeys: [sender nonce initCode callData callGasLimit verificationGasLimit preVerificationGas maxFeePerGas maxPriorityFeePerGas]
  
[DEBUG] Full v07UserOp:
{
  "sender": "0x123...",
  "nonce": "0x0",
  "factory": "0x0000000000400CdFef5E2714E63d8040b700BC24",
  "factoryData": "0x5fbfb9cf...",
  "callData": "0x18dfb3c7...",
  ...
}

[DEBUG] Requesting paymaster data from Alchemy
  Sender: 0x123...
  Nonce: 0x0
  Factory: 0x0000000000400CdFef5E2714E63d8040b700BC24
  CallDataLength: 456
```

### Example Bad Log Output (Missing Fields):
```
[DEBUG] getPaymasterData called with userOp
  UserOpKeys: [callData]
  
[DEBUG] Missing required field in userOp
  MissingField: sender
```

## Quick Checklist

- [ ] All required fields present in userOp
- [ ] CallData is complete (not truncated)
- [ ] Sender address is valid smart account address
- [ ] Nonce matches on-chain state
- [ ] Factory/factoryData correct (if nonce = 0)
- [ ] Gas limits are reasonable (not 0x0)
- [ ] Dummy signature has correct format (starts with 0x00)

## Contact Points

If you see the AA23 error:
1. Check the debug logs for which field is problematic
2. Verify the userOp structure before it reaches `getPaymasterData()`
3. Test with Tenderly simulation first
4. Check Alchemy dashboard for gas policy limits

## Useful Commands

```bash
# View logs in real-time
tail -f /path/to/your/logs | grep DEBUG

# Search for AA23 errors
grep "AA23" /path/to/your/logs

# Find where getPaymasterData is called
grep -n "getPaymasterData" services/*.go
```
