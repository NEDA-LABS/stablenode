#!/bin/bash

# cd to the db_data directory of the script
cd "$(dirname "$0")/db_data" || exit

# Help function to display usage
show_help() {
    echo "Usage: $0 [-h <host>] [-p <port>] [-d <database>] [-U <username>] [-W <password>] [-s <sslmode>]"
    echo
    echo "Options (all optional - will use .env if not provided):"
    echo "  -h    Database host"
    echo "  -p    Database port"
    echo "  -d    Database name"
    echo "  -U    Database username"
    echo "  -W    Database password"
    echo "  -s    SSL mode (disable|allow|prefer|require|verify-ca|verify-full)"
    echo
    echo "Note: Connection parameters are read from ../../.env by default"
    exit 1
}

# Function to safely read .env values
get_env_value() {
    local key=$1
    local file=$2
    # Using perl for more reliable parsing
    local value=$(perl -ne 'print $1 if /^'$key'[\s]*=[\s]*(.*)/' "$file" | sed 's/^["'\'']//' | sed 's/["'\'']$//')
    echo "$value"
}

# Load from .env first
ENV_FILE="../../.env"
if [ -f "$ENV_FILE" ]; then
    echo "Loading database configuration from $ENV_FILE"
    
    # Safely extract values from .env
    DB_HOST=$(get_env_value "DB_HOST" "$ENV_FILE")
    DB_PORT=$(get_env_value "DB_PORT" "$ENV_FILE")
    DB_NAME=$(get_env_value "DB_NAME" "$ENV_FILE")
    DB_USER=$(get_env_value "DB_USER" "$ENV_FILE")
    DB_PASSWORD=$(get_env_value "DB_PASSWORD" "$ENV_FILE")
    DB_SSLMODE=$(get_env_value "SSL_MODE" "$ENV_FILE")

    # Set default port if not specified
    if [ -z "$DB_PORT" ]; then
        DB_PORT="5432"
    fi

    if [ -z "$DB_SSLMODE" ]; then
        DB_SSLMODE="prefer"
    fi
else
    echo "Warning: .env file not found at $ENV_FILE"
fi

# Parse command line arguments (these will override .env values)
while getopts "h:p:d:U:W:s:" opt; do
    case $opt in
        h) DB_HOST="$OPTARG";;
        p) DB_PORT="$OPTARG";;
        d) DB_NAME="$OPTARG";;
        U) DB_USER="$OPTARG";;
        W) DB_PASSWORD="$OPTARG";;
        s) DB_SSLMODE="$OPTARG";;
        ?) show_help;;
    esac
done

# Verify required parameters
if [ -z "$DB_HOST" ] || [ -z "$DB_NAME" ] || [ -z "$DB_USER" ]; then
    echo "Error: Missing required parameters and couldn't find them in .env file"
    echo "Please ensure your .env file contains: DB_HOST, DB_NAME, DB_USER, DB_PASSWORD"
    show_help
fi

# Only prompt for password if not found in .env or CLI
if [ -z "$DB_PASSWORD" ]; then
    read -s -p "Enter database password: " DB_PASSWORD
    echo
fi

# Set PGPASSWORD environment variable
export PGPASSWORD="$DB_PASSWORD"

# Compose shared connection args for psql invocations
PSQL_CONN_ARGS=(
    -h "$DB_HOST"
    -p "$DB_PORT"
    -d "$DB_NAME"
    -U "$DB_USER"
)

# Function to execute SQL files
import_sql() {
    local file="$1"
    echo "Importing $file..."
    PGSSLMODE="$DB_SSLMODE" psql "${PSQL_CONN_ARGS[@]}" \
        --set ON_ERROR_STOP=1 \
        -f "$file"
    
    if [ $? -eq 0 ]; then
        echo "Successfully imported $file"
    else
        echo "Error importing $file"
        exit 1
    fi
}

# Main import process
echo "Starting database import process..."
echo "Using database: $DB_NAME on $DB_HOST:$DB_PORT (sslmode=$DB_SSLMODE)"

# Test connection first
echo "Testing database connection..."
if ! PGSSLMODE="$DB_SSLMODE" psql "${PSQL_CONN_ARGS[@]}" -c '\q'; then
    echo "Error: Could not connect to database. Please check your connection parameters."
    exit 1
fi

# Import main database dump
import_sql "dump.sql"

# Import functions in correct order
import_sql "functions/calculate_total_amount.sql"
import_sql "functions/check_payment_order_amount.sql"

echo "Import process completed successfully"

# Clean up
unset PGPASSWORD

exit 0