# AI Coding Assistant Prompt for Provision Node Development

Use this prompt when working with AI coding assistants (ChatGPT, Claude, etc.) to build your provision node.

---

## Initial Setup Prompt

```
I need to build a provision node service that integrates with a payment aggregator system. 
The provision node will:

1. Receive order assignments from an aggregator
2. Process fiat disbursements to recipients via payment service providers (PSPs)
3. Report order status back to the aggregator
4. Manage balance tracking and updates

Tech Stack:
- Language: Python with Flask (or Node.js with Express)
- Database: SQLite for development, PostgreSQL for production
- Authentication: HMAC-SHA256 signatures
- Deployment: Docker

I have complete specifications in PROVISION_NODE_SPEC.md. Let's start with Milestone 1: 
Project Setup & Core Infrastructure.

Please help me:
1. Create the project structure
2. Set up the Flask application with proper configuration
3. Create the database schema with SQLAlchemy models
4. Set up environment variable management
5. Create a basic Dockerfile

Follow Python best practices and include proper error handling and logging.
```

---

## Milestone-Specific Prompts

### Milestone 1: Project Setup

```
I'm starting a provision node project. Please create:

1. Project structure following Flask best practices:
   - app/ (main application code)
   - models/ (database models)
   - services/ (business logic)
   - utils/ (utilities)
   - tests/ (test files)
   - config.py (configuration)
   - requirements.txt (dependencies)

2. Database models for:
   - Orders table (id, aggregator_order_id, amount, currency, token, status, etc.)
   - Balances table (currency, available, total, reserved)
   - Transactions table (order_id, psp_transaction_id, status, etc.)

3. Basic Flask app with:
   - Configuration from environment variables
   - Database initialization
   - Logging setup
   - Error handlers

4. Dockerfile for containerization

Use SQLAlchemy for ORM and include proper type hints.
```

### Milestone 2: Authentication

```
I need to implement HMAC-SHA256 authentication for my provision node. Please create:

1. A utility function to generate HMAC signatures:
   - Takes payload dict and secret key
   - Adds timestamp to payload
   - Returns hex-encoded signature

2. A Flask decorator to verify incoming HMAC signatures:
   - Extracts Authorization header
   - Parses "HMAC client_id:signature" format
   - Verifies signature matches
   - Checks timestamp is within 5 minutes
   - Returns 401 if invalid

3. A function to make authenticated requests to the aggregator:
   - Generates HMAC signature
   - Adds Authorization header
   - Handles retries with exponential backoff

4. Unit tests for all authentication functions

Include proper error handling and logging.
```

### Milestone 3: API Endpoints

```
I need to implement the core API endpoints for my provision node:

1. GET /health endpoint:
   - Returns: {"status": "success", "message": "Node is live", "data": {"currencies": ["NGN"]}}
   - No authentication required

2. GET /info endpoint:
   - Returns: {"status": "success", "data": {"serviceInfo": {"currencies": ["NGN"], "version": "1.0.0"}}}
   - No authentication required

3. POST /orders endpoint:
   - Receives order from aggregator
   - Validates HMAC signature
   - Validates order data (amount, currency, recipient)
   - Stores order in database with status "pending"
   - Returns: {"status": "success", "data": {"orderId": "...", "status": "pending"}}

Include proper error handling, validation, and logging for each endpoint.
Use Flask blueprints to organize routes.
```

### Milestone 4: Aggregator Communication

```
I need to create a service class to communicate with the aggregator. Please create:

1. AggregatorClient class with methods:
   - accept_order(order_id) - POST /v1/provider/orders/{id}/accept
   - fulfill_order(order_id, transaction_hash) - POST /v1/provider/orders/{id}/fulfill
   - decline_order(order_id, reason) - POST /v1/provider/orders/{id}/decline
   - cancel_order(order_id, reason) - POST /v1/provider/orders/{id}/cancel
   - update_balance(balances) - POST /v1/provider/balances

2. Each method should:
   - Generate HMAC signature with timestamp
   - Make HTTP request to aggregator
   - Handle errors and retries (max 3 retries with exponential backoff)
   - Log request/response
   - Return success/failure status

3. Configuration from environment variables:
   - AGGREGATOR_BASE_URL
   - AGGREGATOR_CLIENT_ID
   - AGGREGATOR_SECRET_KEY

Include comprehensive error handling and logging.
```

### Milestone 5: PSP Integration

```
I need to integrate with Lenco payment service provider. Please create:

1. LencoClient class with methods:
   - initiate_payment(order) - Creates transfer via Lenco API
   - check_payment_status(transaction_id) - Gets transfer status
   - verify_webhook_signature(payload, signature) - Verifies webhook

2. POST /webhooks/psp endpoint:
   - Receives webhook from Lenco
   - Verifies signature
   - Updates order status in database
   - Triggers order fulfillment if successful

3. Configuration from environment variables:
   - LENCO_BASE_URL
   - LENCO_API_KEY
   - LENCO_ACCOUNT_ID

4. Error handling for:
   - Network timeouts
   - Invalid responses
   - Insufficient balance
   - Invalid account details

Include retry logic and comprehensive logging.
```

### Milestone 6: Order Processing

```
I need to implement the order processing pipeline. Please create:

1. OrderProcessor class with methods:
   - process_order(order_id) - Main processing logic
   - validate_order(order) - Checks balance, limits, expiration
   - accept_order(order_id) - Accepts order with aggregator
   - initiate_payment(order_id) - Starts PSP payment
   - fulfill_order(order_id, tx_hash) - Marks order fulfilled
   - cancel_order(order_id, reason) - Cancels order

2. Background job using APScheduler:
   - Polls for pending orders every 30 seconds
   - Checks PSP status for processing orders every 10 seconds
   - Handles order expiration
   - Updates balances every 5 minutes

3. Order state machine:
   - pending → accepted → processing → fulfilled
   - pending → declined
   - processing → cancelled (if PSP fails)

4. Balance management:
   - Reserve balance when accepting order
   - Release balance when order completes/cancels

Include comprehensive error handling and state transition logging.
```

### Milestone 7: Testing

```
I need comprehensive tests for my provision node. Please create:

1. Unit tests for:
   - HMAC signature generation and verification
   - Order validation logic
   - Balance checking
   - All utility functions

2. Integration tests for:
   - API endpoints (health, info, orders)
   - Aggregator client methods
   - PSP client methods
   - Database operations

3. End-to-end test:
   - Receive order → Accept → Initiate payment → Fulfill
   - Mock aggregator and PSP responses

4. Test fixtures and factories for:
   - Order objects
   - Balance objects
   - PSP responses

Use pytest and include proper mocking for external services.
Aim for >80% code coverage.
```

### Milestone 8: Deployment

```
I need to prepare my provision node for production deployment. Please help me:

1. Create production-ready Dockerfile:
   - Multi-stage build
   - Non-root user
   - Health check
   - Proper signal handling

2. Create docker-compose.yml:
   - Provision node service
   - PostgreSQL database
   - Volume mounts for data persistence
   - Environment variable configuration
   - Network configuration for aggregator communication

3. Create deployment documentation:
   - Environment variables reference
   - Database migration steps
   - Docker deployment instructions
   - Health check verification

4. Add production configurations:
   - Gunicorn for WSGI server
   - Proper logging configuration
   - Error tracking setup

Include security best practices and monitoring setup.
```

---

## Debugging Prompts

### Authentication Issues

```
I'm having issues with HMAC authentication. The aggregator is rejecting my requests with 401 Unauthorized.

My signature generation code:
[paste your code]

The error I'm getting:
[paste error]

Please help me:
1. Debug the signature generation
2. Verify the payload format is correct
3. Check timestamp handling
4. Ensure the signature matches what the aggregator expects

The aggregator expects:
- Authorization header: "HMAC {client_id}:{signature}"
- Payload must include timestamp (unix timestamp)
- Signature is HMAC-SHA256 hex-encoded
- Timestamp must be within 5 minutes
```

### Order Processing Issues

```
Orders are getting stuck in "processing" state and not being fulfilled.

Current flow:
1. Order received from aggregator
2. Order accepted successfully
3. PSP payment initiated
4. PSP returns success
5. But order is not being marked as fulfilled

Relevant code:
[paste your code]

Logs:
[paste relevant logs]

Please help me:
1. Identify where the flow is breaking
2. Check if fulfill_order is being called
3. Verify the aggregator fulfill endpoint is being reached
4. Add better error handling and logging
```

### Balance Sync Issues

```
The balance in the aggregator doesn't match my local balance.

Local balance: 50000 NGN
Aggregator balance: 45000 NGN

My balance update code:
[paste your code]

Please help me:
1. Debug the balance calculation
2. Check if balance updates are reaching the aggregator
3. Implement balance reconciliation logic
4. Add audit logging for balance changes
```

---

## Code Review Prompt

```
Please review my provision node code for:

1. Security issues:
   - HMAC signature handling
   - Input validation
   - Secret management
   - SQL injection vulnerabilities

2. Code quality:
   - Error handling
   - Logging
   - Code organization
   - Type hints

3. Performance:
   - Database queries
   - API calls
   - Background jobs

4. Best practices:
   - Python conventions (PEP 8)
   - Flask best practices
   - Database transactions
   - Testing coverage

Code to review:
[paste your code or provide file structure]

Please provide specific recommendations for improvements.
```

---

## Optimization Prompt

```
My provision node is experiencing performance issues:

Issue: [describe the issue]
- Slow response times
- High memory usage
- Database bottlenecks
- etc.

Current metrics:
- Average response time: [X]ms
- Orders per minute: [X]
- Database query time: [X]ms

Please help me:
1. Identify performance bottlenecks
2. Optimize database queries
3. Improve API response times
4. Reduce memory usage
5. Add caching where appropriate

Relevant code:
[paste code sections]
```

---

## Feature Addition Prompt

```
I want to add [feature name] to my provision node.

Feature description:
[describe what you want to add]

Requirements:
- [requirement 1]
- [requirement 2]
- [requirement 3]

Current architecture:
[describe relevant parts of your current code]

Please help me:
1. Design the feature implementation
2. Identify what needs to be changed
3. Write the code for the new feature
4. Add tests for the new feature
5. Update documentation

Maintain consistency with existing code style and patterns.
```

---

## Tips for Using These Prompts

1. **Be Specific**: Include relevant code snippets, error messages, and logs
2. **Provide Context**: Explain what you've tried and what didn't work
3. **Ask for Explanations**: Request explanations of the code, not just code
4. **Iterate**: Start with a basic implementation, then refine
5. **Test Incrementally**: Test each component before moving to the next
6. **Document**: Ask for documentation and comments in the code
7. **Follow Standards**: Request adherence to language-specific best practices
8. **Security First**: Always ask about security implications
9. **Error Handling**: Request comprehensive error handling
10. **Logging**: Ask for detailed logging for debugging

---

## Example Conversation Flow

```
You: [Use Initial Setup Prompt]

AI: [Provides project structure and code]

You: "Thanks! Now let's implement the HMAC authentication. [Use Milestone 2 prompt]"

AI: [Provides authentication code]

You: "I'm getting an error when verifying signatures: [paste error]. Can you help debug?"

AI: [Helps debug]

You: "Great! Now let's move to Milestone 3. [Use Milestone 3 prompt]"

[Continue through milestones]
```

---

## Additional Resources to Provide to AI

When asking for help, you can reference:
- `PROVISION_NODE_SPEC.md` - Complete technical specification
- `PROVISION_NODE_MILESTONES.md` - Development milestones and timeline
- `PROVIDER_SETUP.md` - Provider configuration guide
- Aggregator API documentation
- PSP (Lenco) API documentation

Example:
```
"I'm working on Milestone 4. Please refer to the AggregatorClient specification 
in PROVISION_NODE_SPEC.md under 'Communication with Aggregator' section. 
I need help implementing the accept_order method."
```

---

## Success Checklist

Before considering a milestone complete, verify:
- [ ] Code follows best practices
- [ ] All functions have docstrings
- [ ] Error handling is comprehensive
- [ ] Logging is detailed and useful
- [ ] Tests are written and passing
- [ ] Documentation is updated
- [ ] Code is reviewed
- [ ] Feature works end-to-end

---

Good luck with your provision node development! Use these prompts as starting points and adapt them to your specific needs.
