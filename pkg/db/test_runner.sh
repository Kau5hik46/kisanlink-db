#!/bin/bash

# Database Manager Integration Test Runner
# This script helps run integration tests for different environments

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to show usage
show_usage() {
    echo "Database Manager Integration Test Runner"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -e, --environment ENV    Test environment (local, beta, prod)"
    echo "  -l, --log-level LEVEL    Log level (debug, info, warn, error)"
    echo "  -t, --timeout SECONDS    Test timeout in seconds"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -e local -l debug                    # Run local tests with debug logging"
    echo "  $0 -e beta -l info                      # Run beta tests with info logging"
    echo "  $0 --environment prod --log-level warn  # Run production tests with warn logging"
    echo ""
    echo "Environment Variables:"
    echo "  RUN_INTEGRATION_TESTS=true              # Enable integration tests"
    echo "  TEST_ENVIRONMENT=ENV                    # Set test environment"
    echo "  DB_LOG_LEVEL=LEVEL                      # Set database log level"
    echo ""
}

# Default values
ENVIRONMENT="local"
LOG_LEVEL="info"
TIMEOUT="60"
RUN_TESTS="true"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -l|--log-level)
            LOG_LEVEL="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_usage
            exit 1
            ;;
    esac
done

# Validate environment
case $ENVIRONMENT in
    local|beta|prod)
        ;;
    *)
        print_error "Invalid environment: $ENVIRONMENT"
        print_error "Valid environments: local, beta, prod"
        exit 1
        ;;
esac

# Validate log level
case $LOG_LEVEL in
    debug|info|warn|error)
        ;;
    *)
        print_error "Invalid log level: $LOG_LEVEL"
        print_error "Valid log levels: debug, info, warn, error"
        exit 1
        ;;
esac

# Print test configuration
print_info "Starting Database Manager Integration Tests"
print_info "Environment: $ENVIRONMENT"
print_info "Log Level: $LOG_LEVEL"
print_info "Timeout: ${TIMEOUT}s"
echo ""

# Set environment variables
export RUN_INTEGRATION_TESTS=true
export TEST_ENVIRONMENT=$ENVIRONMENT
export DB_LOG_LEVEL=$LOG_LEVEL

# Load environment-specific configuration
load_environment_config() {
    local env=$1
    
    case $env in
        local)
            print_info "Loading local environment configuration..."
            # Local PostgreSQL (Docker)
            export DB_PRIMARY_BACKEND=gorm
            export DB_POSTGRES_HOST=localhost
            export DB_POSTGRES_PORT=5432
            export DB_POSTGRES_USER=postgres
            export DB_POSTGRES_PASSWORD=password
            export DB_POSTGRES_DBNAME=kisanlink_test
            export DB_POSTGRES_SSLMODE=disable
            export DB_POSTGRES_MAX_CONNS=5
            export DB_POSTGRES_IDLE_CONNS=2
            export DB_POSTGRES_READ_REPLICAS=localhost:5433
            
            # Local DynamoDB (DynamoDB Local)
            export DB_DYNAMO_REGION=us-east-1
            export DB_DYNAMO_TABLE=kisanlink_test
            
            # Local SpiceDB (Docker)
            export DB_SPICEDB_ENDPOINT=localhost:50051
            export DB_SPICEDB_TOKEN=test-token
            ;;
            
        beta)
            print_info "Loading beta environment configuration..."
            # Beta environment variables should be set externally
            # or loaded from a configuration file
            print_warning "Beta environment configuration not set"
            print_warning "Please set the following environment variables:"
            print_warning "  DB_PRIMARY_BACKEND, DB_POSTGRES_*, DB_DYNAMO_*, DB_SPICEDB_*"
            ;;
            
        prod)
            print_info "Loading production environment configuration..."
            print_warning "Production environment configuration not set"
            print_warning "Please set the following environment variables:"
            print_warning "  DB_PRIMARY_BACKEND, DB_POSTGRES_*, DB_DYNAMO_*, DB_SPICEDB_*"
            ;;
    esac
}

# Check if required tools are available
check_prerequisites() {
    print_info "Checking prerequisites..."
    
    # Check if Go is available
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    # Check if we're in a Go module
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Please run this script from the project root"
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Run the tests
run_tests() {
    print_info "Running integration tests..."
    
    # Change to the db package directory
    cd pkg/db
    
    # Run the specific test based on environment
    case $ENVIRONMENT in
        local)
            print_info "Running local integration tests..."
            go test -v -run TestIntegrationLocal -timeout ${TIMEOUT}s
            ;;
        beta)
            print_info "Running beta integration tests..."
            go test -v -run TestIntegrationBeta -timeout ${TIMEOUT}s
            ;;
        prod)
            print_info "Running production integration tests..."
            go test -v -run TestIntegration -timeout ${TIMEOUT}s
            ;;
    esac
    
    if [ $? -eq 0 ]; then
        print_success "Integration tests completed successfully!"
    else
        print_error "Integration tests failed!"
        exit 1
    fi
}

# Main execution
main() {
    print_info "Database Manager Integration Test Runner"
    echo ""
    
    check_prerequisites
    load_environment_config $ENVIRONMENT
    run_tests
    
    print_success "All tests completed!"
}

# Run main function
main "$@" 