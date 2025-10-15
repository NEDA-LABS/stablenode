#!/bin/bash
# Export DATABASE_URL from .env file
# Usage: source pool_management/scripts/export_db_url.sh

# Get the project root (parent of pool_management)
PROJECT_ROOT="/home/commendatore/Desktop/NEDA/rails/aggregator"

# Check if .env exists
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "❌ ERROR: .env file not found at $PROJECT_ROOT/.env"
    echo ""
    echo "Please create .env file with database credentials:"
    echo "  DB_NAME=your_db_name"
    echo "  DB_USER=your_db_user"
    echo "  DB_PASSWORD=your_password"
    echo "  DB_HOST=your_host"
    echo "  DB_PORT=5432"
    echo "  SSL_MODE=require"
    return 1 2>/dev/null || exit 1
fi

# Load environment variables from .env
set -a
source "$PROJECT_ROOT/.env"
set +a

# Extract database variables
DB_NAME="${DB_NAME}"
DB_USER="${DB_USER}"
DB_PASSWORD="${DB_PASSWORD}"
DB_HOST="${DB_HOST}"
DB_PORT="${DB_PORT:-5432}"
SSL_MODE="${SSL_MODE:-require}"

# Check if required variables are set
if [ -z "$DB_NAME" ] || [ -z "$DB_USER" ] || [ -z "$DB_PASSWORD" ] || [ -z "$DB_HOST" ]; then
    echo "❌ ERROR: Missing required database variables in .env"
    echo ""
    echo "Required variables:"
    echo "  DB_NAME: ${DB_NAME:-NOT SET}"
    echo "  DB_USER: ${DB_USER:-NOT SET}"
    echo "  DB_PASSWORD: ${DB_PASSWORD:-NOT SET}"
    echo "  DB_HOST: ${DB_HOST:-NOT SET}"
    echo "  DB_PORT: ${DB_PORT}"
    echo "  SSL_MODE: ${SSL_MODE}"
    return 1 2>/dev/null || exit 1
fi

# Construct DATABASE_URL
export DATABASE_URL="postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${SSL_MODE}"

# Show success message
echo "✓ DATABASE_URL exported successfully"
echo ""
echo "Database connection details:"
echo "  Host: ${DB_HOST}"
echo "  Port: ${DB_PORT}"
echo "  Database: ${DB_NAME}"
echo "  User: ${DB_USER}"
echo "  SSL Mode: ${SSL_MODE}"
echo ""
echo "DATABASE_URL is now available in your shell session."
echo ""
echo "To verify connection:"
echo "  psql \"\$DATABASE_URL\" -c '\\conninfo'"
echo ""
