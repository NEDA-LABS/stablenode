#!/bin/bash

# Alchemy Webhook Test Runner
# Usage: ./run_tests.sh [unit|integration|e2e|all|coverage]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test directories
UNIT_TESTS="./tests/unit/..."
INTEGRATION_TESTS="./tests/integration/..."
E2E_TESTS="./tests/e2e/..."
SERVICE_TESTS="./services/alchemy_webhook_test.go"

# Functions
print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

run_unit_tests() {
    print_header "Running Unit Tests"
    if go test $UNIT_TESTS -v -short; then
        print_success "Unit tests passed"
        return 0
    else
        print_error "Unit tests failed"
        return 1
    fi
}

run_integration_tests() {
    print_header "Running Integration Tests"
    if go test $INTEGRATION_TESTS -v; then
        print_success "Integration tests passed"
        return 0
    else
        print_error "Integration tests failed"
        return 1
    fi
}

run_e2e_tests() {
    print_header "Running End-to-End Tests"
    print_warning "E2E tests require running application and database"
    
    # Check if application is running
    if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
        print_error "Application not running on localhost:8000"
        print_warning "Start application first: docker-compose up -d"
        return 1
    fi
    
    if go test $E2E_TESTS -v; then
        print_success "E2E tests passed"
        return 0
    else
        print_error "E2E tests failed"
        return 1
    fi
}

run_service_tests() {
    print_header "Running Service Tests"
    if go test $SERVICE_TESTS -v; then
        print_success "Service tests passed"
        return 0
    else
        print_error "Service tests failed"
        return 1
    fi
}

run_all_tests() {
    print_header "Running All Tests"
    
    local failed=0
    
    run_unit_tests || failed=1
    echo ""
    
    run_service_tests || failed=1
    echo ""
    
    run_integration_tests || failed=1
    echo ""
    
    print_warning "Skipping E2E tests (run separately with: ./run_tests.sh e2e)"
    
    if [ $failed -eq 0 ]; then
        print_success "All tests passed!"
        return 0
    else
        print_error "Some tests failed"
        return 1
    fi
}

run_coverage() {
    print_header "Running Tests with Coverage"
    
    local coverage_file="coverage.out"
    local coverage_html="coverage.html"
    
    echo "Generating coverage report..."
    if go test ./tests/... ./services/alchemy_webhook_test.go -coverprofile=$coverage_file -covermode=atomic; then
        print_success "Tests completed"
        
        # Generate HTML report
        go tool cover -html=$coverage_file -o $coverage_html
        
        # Show coverage summary
        echo ""
        print_header "Coverage Summary"
        go tool cover -func=$coverage_file | tail -n 1
        
        print_success "Coverage report generated: $coverage_html"
        print_warning "Open $coverage_html in browser to view detailed coverage"
        
        return 0
    else
        print_error "Tests failed"
        return 1
    fi
}

run_quick() {
    print_header "Running Quick Tests (Unit + Service)"
    
    local failed=0
    
    run_unit_tests || failed=1
    echo ""
    
    run_service_tests || failed=1
    
    if [ $failed -eq 0 ]; then
        print_success "Quick tests passed!"
        return 0
    else
        print_error "Quick tests failed"
        return 1
    fi
}

run_specific() {
    local test_name=$1
    print_header "Running Specific Test: $test_name"
    
    if go test ./tests/... -v -run "$test_name"; then
        print_success "Test passed: $test_name"
        return 0
    else
        print_error "Test failed: $test_name"
        return 1
    fi
}

show_help() {
    echo "Alchemy Webhook Test Runner"
    echo ""
    echo "Usage: ./run_tests.sh [command]"
    echo ""
    echo "Commands:"
    echo "  unit          Run unit tests only"
    echo "  integration   Run integration tests only"
    echo "  e2e           Run end-to-end tests only"
    echo "  service       Run service tests only"
    echo "  all           Run all tests (except E2E)"
    echo "  quick         Run unit + service tests (fastest)"
    echo "  coverage      Run tests with coverage report"
    echo "  specific      Run specific test by name"
    echo "  help          Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./run_tests.sh unit"
    echo "  ./run_tests.sh coverage"
    echo "  ./run_tests.sh specific TestAlchemyWebhookHandler"
    echo ""
}

# Main script
case "${1:-all}" in
    unit)
        run_unit_tests
        ;;
    integration)
        run_integration_tests
        ;;
    e2e)
        run_e2e_tests
        ;;
    service)
        run_service_tests
        ;;
    all)
        run_all_tests
        ;;
    quick)
        run_quick
        ;;
    coverage)
        run_coverage
        ;;
    specific)
        if [ -z "$2" ]; then
            print_error "Please provide test name"
            echo "Usage: ./run_tests.sh specific TestName"
            exit 1
        fi
        run_specific "$2"
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac

exit $?
