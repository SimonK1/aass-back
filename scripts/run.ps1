#!/bin/zsh

# Default to "start" if no command argument is provided
command=${1:-start}

# Set the ProjectRoot to the parent directory of the script location
ProjectRoot="$(cd "$(dirname "$0")/.." && pwd)"

# Set environment variables
export AMBULANCE_API_ENVIRONMENT="Development"
export AMBULANCE_API_PORT="8080"
export AMBULANCE_API_MONGODB_USERNAME="root"
export AMBULANCE_API_MONGODB_PASSWORD="neUhaDnes"

# Define a helper function to call docker compose with the proper compose file
mongo() {
    docker compose --file "${ProjectRoot}/deployments/docker-compose/compose.yaml" "$@"
}

case "$command" in
    openapi)
        docker run --rm -ti -v "${ProjectRoot}:/local" openapitools/openapi-generator-cli generate -c /local/scripts/generator-cfg.yaml
        ;;
    start)
        # Bring up mongo in detached mode
        mongo up --detach
        # Ensure that mongo down is run when the script exits, even if go run fails
        trap 'mongo down' EXIT
        go run "${ProjectRoot}/cmd/ambulance-api-service"
        # Remove the trap (optional, if additional commands are run after)
        trap - EXIT
        ;;
    mongo)
        mongo up
        ;;
    *)
        echo "Unknown command: $command" >&2
        exit 1
        ;;
esac
