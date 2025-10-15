# IDE Cache Issue - GetContractEventsWithFallback

## Problem
Your IDE is showing this error:
```
s.alchemyService.GetContractEventsWithFallback undefined (type *services.AlchemyService has no field or method GetContractEventsWithFallback)
```

## Root Cause
This is a **false positive** caused by your IDE's Language Server Protocol (LSP/gopls) cache being stale. The method **does exist** in the codebase.

## Evidence
The method `GetContractEventsWithFallback` is properly defined in:
- **File**: `/home/commendatore/Desktop/NEDA/rails/aggregator/services/alchemy.go`
- **Line**: 1531-1538
- **Signature**: 
```go
func (s *AlchemyService) GetContractEventsWithFallback(
    ctx context.Context, 
    network *ent.Network, 
    contractAddress string, 
    fromBlock int64, 
    toBlock int64, 
    topics []string, 
    txHash string, 
    eventPayload map[string]string
) ([]interface{}, error)
```

The method is used in:
- `services/indexer/evm.go` line 82
- `services/indexer/evm.go` line 102
- `services/indexer/evm.go` line 571
- `services/indexer/evm.go` line 851

## Verification
The code **compiles successfully**:
```bash
$ go build
# No errors - build succeeds
```

## Solutions

### Option 1: Restart Go Language Server (Recommended)
If using VS Code:
1. Press `Ctrl+Shift+P` (or `Cmd+Shift+P` on Mac)
2. Type "Go: Restart Language Server"
3. Select it and press Enter

If using other IDEs:
- **GoLand/IntelliJ**: File → Invalidate Caches → Invalidate and Restart
- **Vim/Neovim with gopls**: Restart your LSP client (`:LspRestart` or similar)
- **Emacs with lsp-mode**: `M-x lsp-workspace-restart`

### Option 2: Clean and Rebuild
```bash
go clean -cache
go build
```

### Option 3: Force gopls to reindex
```bash
# Remove gopls cache
rm -rf ~/.cache/gopls
# Or on macOS:
rm -rf ~/Library/Caches/gopls
```

### Option 4: Reload IDE Window
- **VS Code**: Reload window (`Ctrl+Shift+P` → "Developer: Reload Window")
- **Other IDEs**: Close and reopen the project

## Conclusion
**No code changes are needed**. The method exists and the code compiles. This is purely an IDE cache issue that will be resolved by refreshing your language server.
