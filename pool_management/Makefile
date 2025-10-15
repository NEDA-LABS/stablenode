# Makefile for Receive Address Pool Management
# Usage: make [target]

# Configuration
NETWORK ?= base-sepolia
CHAIN_ID ?= 84532
COUNT ?= 10
RPC_URL ?= $(BASE_SEPOLIA_RPC)
PRIVATE_KEY ?= $(DEPLOYER_PRIVATE_KEY)
OWNER ?= 0xFb84E5503bD20526f2579193411Dd0993d08077519b6f7

# File names
POOL_FILE = pool_$(NETWORK)_$(shell date +%Y%m%d_%H%M%S).json
DEPLOY_RESULTS = deployment_$(NETWORK)_$(shell date +%Y%m%d_%H%M%S).json

# Colors for output
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[1;33m
NC = \033[0m # No Color

.PHONY: help build create deploy mark-deployed full-deploy verify clean

help: ## Show this help message
	@echo "$(GREEN)Receive Address Pool Management$(NC)"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make -f Makefile.pool [target] [options]"
	@echo ""
	@echo "$(YELLOW)Targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Options:$(NC)"
	@echo "  NETWORK=<name>       Network identifier (default: base-sepolia)"
	@echo "  CHAIN_ID=<id>        Chain ID (default: 84532)"
	@echo "  COUNT=<n>            Number of addresses (default: 10)"
	@echo "  RPC_URL=<url>        RPC endpoint URL"
	@echo "  PRIVATE_KEY=<key>    Private key for deployment"
	@echo "  OWNER=<address>      Owner address for smart accounts"
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make full-deploy NETWORK=base-sepolia COUNT=10"
	@echo "  make create NETWORK=ethereum-mainnet CHAIN_ID=1"
	@echo "  make verify NETWORK=base-sepolia"

build: ## Build all pool management binaries
	@echo "$(GREEN)Building pool management tools...$(NC)"
	@cd .. && go build -o pool_management/bin/create_receive_pool ./pool_management/cmd/create_receive_pool
	@cd .. && go build -o pool_management/bin/deploy_pool_addresses ./pool_management/cmd/deploy_pool_addresses
	@cd .. && go build -o pool_management/bin/mark_deployed ./pool_management/cmd/mark_deployed
	@echo "$(GREEN)✓ Build complete$(NC)"

create: build ## Create addresses (NETWORK, CHAIN_ID, COUNT, OWNER)
	@echo "$(GREEN)Creating $(COUNT) addresses for $(NETWORK) (Chain ID: $(CHAIN_ID))...$(NC)"
	@cd .. && ./pool_management/bin/create_receive_pool \
		--count $(COUNT) \
		--chain-id $(CHAIN_ID) \
		--network $(NETWORK) \
		--owner $(OWNER) \
		--output pool_management/$(POOL_FILE) \
		--save-db
	@echo "$(GREEN)✓ Addresses created: $(POOL_FILE)$(NC)"

create-no-db: build ## Create addresses without saving to database
	@echo "$(GREEN)Creating $(COUNT) addresses for $(NETWORK) (no DB save)...$(NC)"
	@cd .. && ./pool_management/bin/create_receive_pool \
		--count $(COUNT) \
		--chain-id $(CHAIN_ID) \
		--network $(NETWORK) \
		--owner $(OWNER) \
		--output pool_management/$(POOL_FILE)
	@echo "$(GREEN)✓ Addresses created: $(POOL_FILE)$(NC)"

deploy: build ## Deploy addresses (requires POOL_FILE, RPC_URL, PRIVATE_KEY)
	@if [ -z "$(POOL_FILE_INPUT)" ]; then \
		echo "$(RED)ERROR: POOL_FILE_INPUT not set$(NC)"; \
		echo "Usage: make -f Makefile.pool deploy POOL_FILE_INPUT=pool_xxx.json"; \
		exit 1; \
	fi
	@if [ -z "$(RPC_URL)" ]; then \
		echo "$(RED)ERROR: RPC_URL not set$(NC)"; \
		exit 1; \
	fi
	@if [ -z "$(PRIVATE_KEY)" ]; then \
		echo "$(RED)ERROR: PRIVATE_KEY not set$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Deploying addresses from $(POOL_FILE_INPUT)...$(NC)"
	@./bin/deploy_pool_addresses \
		--input $(POOL_FILE_INPUT) \
		--private-key $(PRIVATE_KEY) \
		--rpc-url $(RPC_URL) \
		--output $(DEPLOY_RESULTS)
	@echo "$(GREEN)✓ Deployment complete: $(DEPLOY_RESULTS)$(NC)"

deploy-dry-run: build ## Dry run deployment (doesn't send transactions)
	@if [ -z "$(POOL_FILE_INPUT)" ]; then \
		echo "$(RED)ERROR: POOL_FILE_INPUT not set$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Dry run deployment from $(POOL_FILE_INPUT)...$(NC)"
	@./bin/deploy_pool_addresses \
		--input $(POOL_FILE_INPUT) \
		--private-key $(PRIVATE_KEY) \
		--rpc-url $(RPC_URL) \
		--dry-run

mark-deployed: build ## Mark addresses as deployed (requires DEPLOY_RESULTS_INPUT)
	@if [ -z "$(DEPLOY_RESULTS_INPUT)" ]; then \
		echo "$(RED)ERROR: DEPLOY_RESULTS_INPUT not set$(NC)"; \
		echo "Usage: make -f Makefile.pool mark-deployed DEPLOY_RESULTS_INPUT=deployment_xxx.json"; \
		exit 1; \
	fi
	@echo "$(GREEN)Marking addresses as deployed from $(DEPLOY_RESULTS_INPUT)...$(NC)"
	@./bin/mark_deployed \
		--input $(DEPLOY_RESULTS_INPUT) \
		--status pool_ready
	@echo "$(GREEN)✓ Database updated$(NC)"

mark-deployed-dry-run: build ## Dry run mark deployed (doesn't update database)
	@if [ -z "$(DEPLOY_RESULTS_INPUT)" ]; then \
		echo "$(RED)ERROR: DEPLOY_RESULTS_INPUT not set$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Dry run marking addresses from $(DEPLOY_RESULTS_INPUT)...$(NC)"
	@./bin/mark_deployed \
		--input $(DEPLOY_RESULTS_INPUT) \
		--dry-run

full-deploy: create deploy mark-deployed ## Complete flow: create -> deploy -> mark (NETWORK, COUNT, RPC_URL, PRIVATE_KEY)
	@echo "$(GREEN)✓✓✓ Full deployment complete for $(NETWORK)$(NC)"
	@echo "$(YELLOW)Files created:$(NC)"
	@echo "  - $(POOL_FILE)"
	@echo "  - $(DEPLOY_RESULTS)"

verify: ## Verify pool status in database (NETWORK)
	@echo "$(GREEN)Verifying pool status for $(NETWORK)...$(NC)"
	@echo "$(YELLOW)Pool Status:$(NC)"
	@psql $(DATABASE_URL) -c "\
		SELECT \
			status, \
			COUNT(*) as count, \
			AVG(times_used) as avg_times_used \
		FROM receive_addresses \
		WHERE network_identifier = '$(NETWORK)' \
		AND is_deployed = true \
		GROUP BY status \
		ORDER BY status;" 2>/dev/null || echo "$(RED)Database connection failed$(NC)"

verify-address: ## Verify specific address on-chain (requires ADDRESS, RPC_URL)
	@if [ -z "$(ADDRESS)" ]; then \
		echo "$(RED)ERROR: ADDRESS not set$(NC)"; \
		echo "Usage: make -f Makefile.pool verify-address ADDRESS=0x... RPC_URL=..."; \
		exit 1; \
	fi
	@if [ -z "$(RPC_URL)" ]; then \
		echo "$(RED)ERROR: RPC_URL not set$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Verifying address $(ADDRESS) on-chain...$(NC)"
	@CODE=$$(cast code $(ADDRESS) --rpc-url $(RPC_URL) 2>/dev/null); \
	if [ "$$CODE" = "0x" ]; then \
		echo "$(RED)✗ Address NOT deployed (no code)$(NC)"; \
	else \
		echo "$(GREEN)✓ Address IS deployed$(NC)"; \
		echo "$(YELLOW)Code length: $${#CODE} characters$(NC)"; \
	fi

list-files: ## List all pool-related files
	@echo "$(GREEN)Pool files:$(NC)"
	@ls -lh pool_*.json 2>/dev/null || echo "No pool files found"
	@echo ""
	@echo "$(GREEN)Deployment result files:$(NC)"
	@ls -lh deployment_*.json 2>/dev/null || echo "No deployment result files found"

clean: ## Remove generated files
	@echo "$(YELLOW)Removing generated files...$(NC)"
	@rm -f pool_*.json deployment_*.json
	@echo "$(GREEN)✓ Clean complete$(NC)"

clean-bin: ## Remove built binaries
	@echo "$(YELLOW)Removing built binaries...$(NC)"
	@rm -f bin/create_receive_pool bin/deploy_pool_addresses bin/mark_deployed
	@echo "$(GREEN)✓ Binaries removed$(NC)"

# Network-specific shortcuts
base-sepolia: ## Deploy 10 addresses to Base Sepolia
	@$(MAKE) -f Makefile.pool full-deploy \
		NETWORK=base-sepolia \
		CHAIN_ID=84532 \
		COUNT=10

base-mainnet: ## Deploy 10 addresses to Base Mainnet
	@$(MAKE) -f Makefile.pool full-deploy \
		NETWORK=base-mainnet \
		CHAIN_ID=8453 \
		COUNT=10

ethereum-sepolia: ## Deploy 10 addresses to Ethereum Sepolia
	@$(MAKE) -f Makefile.pool full-deploy \
		NETWORK=ethereum-sepolia \
		CHAIN_ID=11155111 \
		COUNT=10

# Check environment variables
check-env: ## Check required environment variables
	@echo "$(GREEN)Checking environment variables...$(NC)"
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "$(RED)✗ DATABASE_URL not set$(NC)"; \
	else \
		echo "$(GREEN)✓ DATABASE_URL set$(NC)"; \
	fi
	@if [ -z "$(BASE_SEPOLIA_RPC)" ]; then \
		echo "$(YELLOW)⚠ BASE_SEPOLIA_RPC not set$(NC)"; \
	else \
		echo "$(GREEN)✓ BASE_SEPOLIA_RPC set$(NC)"; \
	fi
	@if [ -z "$(DEPLOYER_PRIVATE_KEY)" ]; then \
		echo "$(YELLOW)⚠ DEPLOYER_PRIVATE_KEY not set$(NC)"; \
	else \
		echo "$(GREEN)✓ DEPLOYER_PRIVATE_KEY set$(NC)"; \
	fi
	@echo ""
	@echo "$(YELLOW)Current settings:$(NC)"
	@echo "  NETWORK: $(NETWORK)"
	@echo "  CHAIN_ID: $(CHAIN_ID)"
	@echo "  COUNT: $(COUNT)"

# Example workflows
example-1: ## Example: Create and save to DB only
	@echo "$(YELLOW)Example 1: Creating addresses and saving to database$(NC)"
	@$(MAKE) -f Makefile.pool create NETWORK=base-sepolia COUNT=5

example-2: ## Example: Create, deploy, and mark complete
	@echo "$(YELLOW)Example 2: Full deployment workflow$(NC)"
	@$(MAKE) -f Makefile.pool check-env
	@$(MAKE) -f Makefile.pool full-deploy NETWORK=base-sepolia COUNT=5

example-3: ## Example: Deploy from existing file
	@echo "$(YELLOW)Example 3: Deploy from existing pool file$(NC)"
	@echo "Make sure to set POOL_FILE_INPUT=your_file.json"
	@$(MAKE) -f Makefile.pool deploy-dry-run POOL_FILE_INPUT=pool_base-sepolia_*.json
