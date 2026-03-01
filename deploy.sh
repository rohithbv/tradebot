#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="tradebot"
IMAGE_TAR="tradebot.tar"
REMOTE_DIR="/opt/tradebot"
PLATFORM="${PLATFORM:-linux/arm64}"

usage() {
    echo "Usage: $0 <user@host>"
    echo ""
    echo "Builds the tradebot Docker image, copies it to a remote host,"
    echo "and starts the container using docker-compose."
    echo ""
    echo "Environment variables:"
    echo "  PLATFORM   Target platform (default: linux/arm64)"
    echo ""
    echo "Examples:"
    echo "  $0 ubuntu@192.168.1.100"
    echo "  PLATFORM=linux/amd64 $0 deploy@myserver.com"
    exit 1
}

if [ $# -ne 1 ]; then
    usage
fi

REMOTE_HOST="$1"

# Validate remote host format
if [[ ! "$REMOTE_HOST" =~ ^[a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+$ ]]; then
    echo "Error: Invalid remote host format. Expected user@host"
    exit 1
fi

echo "==> Building Docker image for ${PLATFORM}..."
docker build --platform "${PLATFORM}" -t "${IMAGE_NAME}:latest" "${SCRIPT_DIR}"

echo "==> Saving image to ${IMAGE_TAR}..."
docker save "${IMAGE_NAME}:latest" -o "${SCRIPT_DIR}/${IMAGE_TAR}"

echo "==> Checking remote directory ${REMOTE_DIR}..."
if ! ssh "${REMOTE_HOST}" "test -d ${REMOTE_DIR} && test -w ${REMOTE_DIR}"; then
    echo "Error: ${REMOTE_DIR} does not exist or is not writable on ${REMOTE_HOST}."
    echo "Create it on the remote host first:"
    echo "  ssh ${REMOTE_HOST} 'sudo mkdir -p ${REMOTE_DIR} && sudo chown \$(whoami):\$(whoami) ${REMOTE_DIR}'"
    rm -f "${SCRIPT_DIR}/${IMAGE_TAR}"
    exit 1
fi

echo "==> Copying files to ${REMOTE_HOST}:${REMOTE_DIR}..."
scp "${SCRIPT_DIR}/${IMAGE_TAR}" \
    "${SCRIPT_DIR}/docker-compose.prod.yml" \
    "${SCRIPT_DIR}/env.example" \
    "${REMOTE_HOST}:${REMOTE_DIR}/"

echo "==> Loading image and starting containers on remote host..."
ssh "${REMOTE_HOST}" bash -s <<'REMOTE_SCRIPT'
set -euo pipefail
cd /opt/tradebot

docker load -i tradebot.tar
rm -f tradebot.tar

# Rename compose file to the default name
mv -f docker-compose.prod.yml docker-compose.yml

# Create .env from example if it doesn't exist
if [ ! -f .env ]; then
    cp env.example .env
    echo ""
    echo "WARNING: .env created from template. Edit ${REMOTE_DIR}/.env with your"
    echo "API credentials before the bot can trade:"
    echo "  ssh ${REMOTE_HOST} nano /opt/tradebot/.env"
    echo ""
fi

docker compose up -d
docker compose ps
REMOTE_SCRIPT

# Clean up local tar
rm -f "${SCRIPT_DIR}/${IMAGE_TAR}"

echo ""
echo "==> Deployment complete!"
echo "    Dashboard: http://${REMOTE_HOST#*@}:8080"
echo "    Logs:      ssh ${REMOTE_HOST} 'cd ${REMOTE_DIR} && docker compose logs -f'"
