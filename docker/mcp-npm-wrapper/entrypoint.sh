#!/bin/bash
set -e

# Determine runtime based on package name or explicit setting
RUNTIME="${MCP_RUNTIME:-auto}"

# Auto-detect runtime if not specified
if [ "$RUNTIME" = "auto" ]; then
    if [[ "$MCP_PACKAGE" == mcp-* ]] || [[ "$MCP_PACKAGE" == mcp_* ]]; then
        # Python packages typically use mcp- or mcp_ prefix
        RUNTIME="python"
    elif [[ "$MCP_PACKAGE" == @* ]]; then
        # NPM scoped packages start with @
        RUNTIME="node"
    else
        # Default to node for unscoped packages
        RUNTIME="node"
    fi
fi

echo "MCP Server Runtime: $RUNTIME"
echo "MCP Package: $MCP_PACKAGE"
echo "MCP Args: $MCP_ARGS"

# Run the appropriate package manager
case "$RUNTIME" in
    node|npm|npx)
        echo "Installing and running Node.js MCP server..."
        exec npx -y "$MCP_PACKAGE" $MCP_ARGS
        ;;
    python|pip|uvx)
        echo "Installing and running Python MCP server..."
        exec uvx "$MCP_PACKAGE" $MCP_ARGS
        ;;
    *)
        echo "Error: Unknown runtime '$RUNTIME'. Use 'node' or 'python'"
        exit 1
        ;;
esac
