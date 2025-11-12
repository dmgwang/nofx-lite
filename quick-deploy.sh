#!/bin/bash
set -e

echo "🚀 Quick deploy starting..."

# Copy environment and config files if they don't exist
if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
  echo "📄 Created .env from .env.example"
fi

if [ ! -f config.json ] && [ -f config.json.example ]; then
  cp config.json.example config.json
  echo "📄 Created config.json from config.json.example"
fi

# Check if Docker is available
if ! command -v docker >/dev/null 2>&1; then
  echo "❌ Docker is not installed or not in PATH"
  exit 1
fi

if ! command -v docker-compose >/dev/null 2>&1 && ! docker compose version >/dev/null 2>&1; then
  echo "❌ Docker Compose is not available"
  exit 1
fi

# Build and start services
echo "🏗️  Building containers (with PostgreSQL)..."
if docker compose build; then
  echo "▶️  Starting containers..."
  docker compose up -d
  echo "✅ Deployment complete! Web interface should be available at http://localhost:3000"
  echo "💡 Use 'docker compose logs -f' to view logs"
else
  echo "❌ Build failed, please check the error messages above"
  exit 1
fi
if ! grep -q '^DATABASE_URL=' .env 2>/dev/null; then
  echo "ℹ️  DATABASE_URL not set in .env; backend will use default: postgres://postgres:postgres@postgres:5432/nofx?sslmode=disable"
fi
