#!/bin/bash
# Quick script to check tokens in database

echo "📊 Checking tokens in Supabase database..."
echo ""

# Load environment variables
source .env

# Use the Supabase SQL API or just show the connection info
echo "Database: $DB_HOST"
echo "Database Name: $DB_NAME"
echo ""
echo "To check tokens, either:"
echo "1. Go to Supabase Dashboard → SQL Editor"
echo "2. Run this query:"
echo ""
echo "SELECT t.id, t.symbol, t.contract_address, t.decimals, n.identifier as network"
echo "FROM tokens t"
echo "JOIN networks n ON t.network_id = n.id"
echo "WHERE n.identifier = 'base-sepolia';"
echo ""
echo "Or use Supabase dashboard: https://supabase.com/dashboard/project/_/editor"
