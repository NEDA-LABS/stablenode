# Paymaster Request Fix - Missing Gas Fields

## Problem Identified

From the logs at `2025-10-13T13:54:54Z`:

```
[DEBUG] Missing required field in userOp
MissingField=callGasLimit
```

The `minimalUserOp` being sent to `getPaymasterData()` only contained:
- ✅ `sender`
- ✅ `nonce`
- ✅ `initCode`
- ✅ `callData`

But was **missing all gas fields**:
- ❌ `callGasLimit`
- ❌ `verificationGasLimit`
- ❌ `preVerificationGas`
- ❌ `maxFeePerGas`
- ❌ `maxPriorityFeePerGas`

## Root Cause

In `services/alchemy.go`, there were two places where the code created a "minimal" UserOp without gas fields before calling the paymaster:

1. **Line 742-747** (for deployment)
2. **Line 933-937** (for regular transactions)

The assumption was that Alchemy's paymaster would work without gas estimates, but **Alchemy requires gas fields to simulate the transaction** and determine if it can be sponsored.

## Solution Applied

### Fix 1: Deployment UserOp (Line 741-754)
**Before:**
```go
minimalUserOp := map[string]interface{}{
    "sender":   userOp["sender"],
    "nonce":    userOp["nonce"],
    "initCode": userOp["initCode"],
    "callData": userOp["callData"],
}
```

**After:**
```go
minimalUserOp := map[string]interface{}{
    "sender":   userOp["sender"],
    "nonce":    userOp["nonce"],
    "initCode": userOp["initCode"],
    "callData": userOp["callData"],
    // Add initial gas estimates (Alchemy will optimize these)
    "callGasLimit":         userOp["callGasLimit"],         // 30k default for deployment
    "verificationGasLimit": userOp["verificationGasLimit"], // 300k for deployment
    "preVerificationGas":   userOp["preVerificationGas"],   // 65536 default
    "maxFeePerGas":         userOp["maxFeePerGas"],         // From gas oracle
    "maxPriorityFeePerGas": userOp["maxPriorityFeePerGas"], // From gas oracle
}
```

### Fix 2: Regular Transaction UserOp (Line 931-944)
**Before:**
```go
minimalUserOp := map[string]interface{}{
    "sender":   userOp["sender"],
    "nonce":    userOp["nonce"],
    "initCode": userOp["initCode"],
    "callData": userOp["callData"],
}
```

**After:**
```go
minimalUserOp := map[string]interface{}{
    "sender":   userOp["sender"],
    "nonce":    userOp["nonce"],
    "initCode": userOp["initCode"],
    "callData": userOp["callData"],
    // Add initial gas estimates (Alchemy will optimize these)
    "callGasLimit":         userOp["callGasLimit"],         // 100k default
    "verificationGasLimit": userOp["verificationGasLimit"], // From earlier calculation
    "preVerificationGas":   userOp["preVerificationGas"],   // 65536 default
    "maxFeePerGas":         userOp["maxFeePerGas"],         // From gas oracle
    "maxPriorityFeePerGas": userOp["maxPriorityFeePerGas"], // From gas oracle
}
```

## How It Works Now

1. **Initial UserOp is built** with default gas estimates (lines 720-737 for deployment, 910-927 for regular)
2. **UserOp with gas fields is sent** to Alchemy's paymaster endpoint
3. **Alchemy simulates** the transaction with the provided gas estimates
4. **Alchemy returns optimized values**:
   - Refined gas limits
   - `paymasterAndData` (sponsorship proof)
5. **Original UserOp is updated** with Alchemy's optimized values (lines 770-787 and 950-969)

## Testing

### Before Fix
```bash
# Log showed:
[DEBUG] Missing required field in userOp
MissingField=callGasLimit
```

### After Fix
You should now see:
```bash
[DEBUG] getPaymasterData called with userOp
  UserOpKeys: [sender nonce initCode callData callGasLimit verificationGasLimit preVerificationGas maxFeePerGas maxPriorityFeePerGas]

[DEBUG] Requesting paymaster data from Alchemy
  Sender: 0x1c0d91c545ee0ccfd19b6b1b738a67504fa86e4f
  CallGasLimit: 0x186a0
  VerificationGasLimit: 0x...
```

## Expected Behavior

With this fix:
1. ✅ All required fields are present in the paymaster request
2. ✅ Alchemy can simulate the transaction properly
3. ✅ Paymaster sponsorship will be approved (if within policy limits)
4. ✅ Gas estimates will be optimized by Alchemy

## Next Steps

1. **Restart your application** to apply the changes
2. **Test with the receive address** `0x1c0d91c545ee0ccfd19b6b1b738a67504fa86e4f`
3. **Check logs** for successful paymaster response:
   ```bash
   grep "Received paymaster and gas data from Alchemy" tmp/logs.txt | tail -5
   ```
4. **Verify the response** includes `paymasterAndData` field

## Related Files Modified

- `services/alchemy.go` (lines 741-754, 931-944)

## Debug Commands

```bash
# Watch for paymaster requests in real-time
tail -f tmp/logs.txt | grep -i "paymaster\|DEBUG"

# Check for the specific receive address
grep "0x1c0d91c545ee0ccfd19b6b1b738a67504fa86e4f" tmp/logs.txt | tail -20

# Look for successful paymaster responses
grep "Received paymaster and gas data" tmp/logs.txt | tail -10
```

## Common Issues After Fix

If you still see errors:

### AA23 Error
- Check that `callData` is complete (not truncated)
- Verify smart account address is correct
- Ensure `initCode` is properly formatted

### AA20 Error
- Account not deployed yet (expected for first transaction)
- Make sure `initCode` is included when nonce = 0

### Policy Limit Errors
- Check Alchemy dashboard for gas policy limits
- Verify the transaction is within policy rules

## Verification Checklist

- [x] Gas fields added to deployment UserOp
- [x] Gas fields added to regular transaction UserOp
- [x] Debug logging enhanced to show all fields
- [x] Error handling improved with detailed analysis
- [ ] Application restarted with new code
- [ ] Test transaction sent to receive address
- [ ] Logs show successful paymaster response
