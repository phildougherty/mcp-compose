# Generic wrapper for MCP servers (Node.js and Python)
FROM python:3.11-slim

# Install Node.js and system dependencies
RUN apt-get update && apt-get install -y \
    nodejs \
    npm \
    tini \
    curl \
    git \
    && rm -rf /var/lib/apt/lists/*

# Upgrade pip and install uv for faster Python installs
RUN pip install --no-cache-dir --upgrade pip uv

# Create a non-root user
RUN groupadd -g 1000 mcp && \
    useradd -m -u 1000 -g mcp mcp

# Set working directory
WORKDIR /app

# Create cache directories
RUN mkdir -p /home/mcp/.npm /home/mcp/.cache/pip && \
    chown -R mcp:mcp /home/mcp

USER mcp

# Use tini as entrypoint for proper signal handling
ENTRYPOINT ["/usr/bin/tini", "--"]

# Entrypoint script to handle both npm and pip packages
COPY --chown=mcp:mcp entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

CMD ["/entrypoint.sh"]
