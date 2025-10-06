-- Migration: Convert NPM command-based servers to Docker wrapper image
-- This updates all servers that use npx to use the generic wrapper image

-- Update all npm-based servers to use the wrapper image
-- The wrapper expects MCP_PACKAGE and MCP_ARGS environment variables

UPDATE marketplace_servers
SET config_template = jsonb_build_object(
    'image', 'localhost:5000/mcpcompose/mcp-server-wrapper:latest',
    'protocol', protocol,
    'env', COALESCE(
        config_template->'env',
        '{}'::jsonb
    ) || jsonb_build_object(
        'MCP_PACKAGE',
        CASE
            WHEN config_template->>'command' = 'npx' THEN
                (config_template->'args'->1)::text
            ELSE config_template->>'command'
        END,
        'MCP_ARGS',
        CASE
            WHEN config_template->>'command' = 'npx'
                AND jsonb_array_length(config_template->'args') > 2
            THEN array_to_string(
                ARRAY(
                    SELECT jsonb_array_elements_text(config_template->'args')
                    OFFSET 2
                ), ' '
            )
            ELSE ''
        END
    )
)
WHERE config_template->>'command' IS NOT NULL;
