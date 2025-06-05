#!/bin/sh
set -e

INTERNAL_PORT="${MCP_SOCAT_INTERNAL_PORT:-12345}"
SERVER_COMMAND="$1"
shift
SERVER_ARGS="$@"

if [ -z "$SERVER_COMMAND" ]; then
    echo "Error: SERVER_COMMAND is not set or provided." >&2
    exit 1
fi

echo "Socat Hoster: Starting socat on TCP port $INTERNAL_PORT to execute: $SERVER_COMMAND $SERVER_ARGS" >&2

# Create a wrapper script that properly handles STDIN
cat > /tmp/mcp_server.sh <<EOF
#!/bin/sh
exec $SERVER_COMMAND $SERVER_ARGS
EOF

chmod +x /tmp/mcp_server.sh

# Use EXEC without shell interpretation - this passes STDIN correctly
exec socat TCP-LISTEN:$INTERNAL_PORT,reuseaddr,fork,bind=0.0.0.0 EXEC:"/tmp/mcp_server.sh",pty,stderr