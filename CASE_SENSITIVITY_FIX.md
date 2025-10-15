# Case Sensitivity & Pool Status Fix

## 🐛 Issues Found

### **Issue 1: Case Sensitivity**
- **Problem**: Blockchain returns lowercase addresses (`0xd76a79fbc6ef2ecb5b0e3a767886d4fd612587aa`)
- **Database**: Stores mixed-case addresses (`0xd76a79FbC6ef2ECb5B0E3a767886D4Fd612587AA`)
- **Result**: Indexer couldn't find orders → marked as "UnknownAddresses"

### **Issue 2: Missing Pool Status**
- **Problem**: Query only checked `status = unused`
- **Pool addresses**: Have `status = pool_assigned`
- **Result**: Pool addresses were ignored by indexer

### **Issue 3: Mnemonic Validation**
- **Problem**: Mnemonic validation ran even when not needed
- **Result**: Spam logs with "Invalid mnemonic phrase"

---

## ✅ Fixes Applied

### **Fix 1: Case-Insensitive Address Matching**

**File**: `services/common/indexer.go` (Lines 68-79)

**Before**:
```go
receiveaddress.AddressIn(unknownAddresses...)
```

**After**:
```go
receiveaddress.Or(
    func(s *sql.Selector) {
        // Case-insensitive address matching
        for i, addr := range unknownAddresses {
            if i == 0 {
                s.Where(sql.EQ(sql.Lower("address"), strings.ToLower(addr)))
            } else {
                s.Or().Where(sql.EQ(sql.Lower("address"), strings.ToLower(addr)))
            }
        }
    },
)
```

**Result**: Now uses `LOWER(address) = LOWER(input)` for matching

---

### **Fix 2: Include Pool Status**

**File**: `services/common/indexer.go` (Lines 63-66)

**Before**:
```go
receiveaddress.StatusEQ(receiveaddress.StatusUnused)
```

**After**:
```go
receiveaddress.Or(
    receiveaddress.StatusEQ(receiveaddress.StatusUnused),
    receiveaddress.StatusEQ(receiveaddress.StatusPoolAssigned),
)
```

**Result**: Now processes both unused AND pool-assigned addresses

---

### **Fix 3: Case-Insensitive Map Lookup**

**File**: `services/common/indexer.go` (Lines 106-115)

**Before**:
```go
transferEvent, ok := addressToEvent[receiveAddress.Address]
```

**After**:
```go
// Case-insensitive lookup in addressToEvent map
var transferEvent *types.TokenTransferEvent
var ok bool
for addr, event := range addressToEvent {
    if strings.EqualFold(addr, receiveAddress.Address) {
        transferEvent = event
        ok = true
        break
    }
}
```

**Result**: Finds events regardless of address case

---

### **Fix 4: Optional Mnemonic Validation**

**File**: `config/config.go` (Lines 55-62)

**Before**:
```go
valid := bip39.IsMnemonicValid(cryptoConf.HDWalletMnemonic)
if !valid {
    fmt.Printf("Invalid mnemonic phrase")
    return nil
}
```

**After**:
```go
// Only validate mnemonic if it's provided (not needed for pool addresses)
if cryptoConf.HDWalletMnemonic != "" {
    valid := bip39.IsMnemonicValid(cryptoConf.HDWalletMnemonic)
    if !valid {
        fmt.Printf("Invalid mnemonic phrase")
        return nil
    }
}
```

**Result**: No more spam logs when mnemonic not configured

---

## 🧪 Testing

### **Before Fix**:
```
INFO ProcessReceiveAddresses called | UnknownAddresses=[0xd76a79fbc6ef2ecb5b0e3a767886d4fd612587aa]
INFO Orders found matching criteria | OrdersFound=0
WARN No transfer event found for receive address
```

### **After Fix** (Expected):
```
INFO ProcessReceiveAddresses called | UnknownAddresses=[0xd76a79fbc6ef2ecb5b0e3a767886d4fd612587aa]
INFO Orders found matching criteria | OrdersFound=1
INFO Updating receive address status | ReceiveAddress=0xd76a79FbC6ef2ECb5B0E3a767886D4Fd612587AA
INFO Successfully updated receive address status
```

---

## 🚀 Deployment

1. **Rebuild the application**:
   ```bash
   make build
   ```

2. **Restart the server**:
   ```bash
   # Stop current server
   # Start new server
   ./stablenode
   ```

3. **Monitor logs**:
   ```bash
   tail -f tmp/logs.txt | grep -i "0xd76a79"
   ```

4. **Verify order updates**:
   ```sql
   SELECT id, receive_address_text, status, amount_paid
   FROM payment_orders
   WHERE LOWER(receive_address_text) = LOWER('0xd76a79FbC6ef2ECb5B0E3a767886D4Fd612587AA');
   ```

---

## 📝 Summary

All three critical issues have been fixed:

✅ **Case sensitivity**: Addresses now match regardless of case
✅ **Pool status**: Pool addresses are now processed by indexer
✅ **Mnemonic spam**: No more "Invalid mnemonic phrase" logs

The payment order for `0xd76a79FbC6ef2ECb5B0E3a767886D4Fd612587AA` should now update correctly when payments are received.
