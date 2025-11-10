#!/bin/bash

# Copy environment and config files if they don't exist
[ ! -f .env ] && [ -f .env.example ] && cp .env.example .env
[ ! -f config.json ] && [ -f config.json.example ] && cp config.json.example config.json

# Build and start services
docker compose build && docker compose up -d

echo "✅ Deployment complete! Web interface should be available at http://localhost:3000"
echo "💡 Use 'docker compose logs -f' to view logs"