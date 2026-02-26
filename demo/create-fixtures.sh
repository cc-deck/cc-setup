#!/usr/bin/env bash
# Generate fake configuration data for demo recording.
# Creates a directory structure that cc-setup reads when HOME and
# XDG_CONFIG_HOME are pointed at it.

set -euo pipefail

FIXTURE_DIR="${1:-$(mktemp -d)/cc-setup-demo}"

echo "Creating demo fixtures in $FIXTURE_DIR"

# ── Directory structure ──────────────────────────────────────────────────────

mkdir -p "$FIXTURE_DIR/home/.config/cc-setup"
mkdir -p "$FIXTURE_DIR/home/.claude"
mkdir -p "$FIXTURE_DIR/workspace/my-project/.claude"

# ── Central server registry (~/.config/cc-setup/mcp.json) ───────────────────
# 9 servers: 1 real (everything via npx, green dot) + 8 fake (red dots).
# Fake servers use 127.0.0.1:1 (instant connection refused) or "false" command.

cat > "$FIXTURE_DIR/home/.config/cc-setup/mcp.json" <<'JSON'
{
  "servers": {
    "confluence": {
      "description": "Confluence wiki",
      "type": "http",
      "url": "http://127.0.0.1:1/mcp"
    },
    "everything": {
      "description": "MCP test server (all features)",
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-everything"]
    },
    "filesystem": {
      "description": "Local filesystem access",
      "type": "stdio",
      "command": "false",
      "args": []
    },
    "github": {
      "description": "GitHub Copilot MCP",
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Bearer ghp_demo_token_12345"
      }
    },
    "google-workspace": {
      "description": "Google Workspace",
      "type": "http",
      "url": "http://127.0.0.1:1/mcp"
    },
    "jira": {
      "description": "Company JIRA",
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Basic ZGVtbzpwYXNz"
      }
    },
    "kubernetes": {
      "description": "K8s cluster management",
      "type": "http",
      "url": "http://127.0.0.1:1/mcp"
    },
    "slack": {
      "description": "Slack workspace",
      "type": "sse",
      "url": "http://127.0.0.1:1/sse",
      "headers": {
        "Authorization": "Bearer xoxb-demo-slack-token"
      }
    },
    "sqlite-db": {
      "description": "SQLite database",
      "type": "stdio",
      "command": "false",
      "args": []
    }
  }
}
JSON
echo "  Central config: 9 servers"

# ── Parent .mcp.json (inherited servers) ─────────────────────────────────────
# github, jira, slack appear as inherited when running from my-project/

cat > "$FIXTURE_DIR/workspace/.mcp.json" <<'JSON'
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Bearer ghp_demo_token_12345"
      }
    },
    "jira": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Basic ZGVtbzpwYXNz"
      }
    },
    "slack": {
      "type": "sse",
      "url": "http://127.0.0.1:1/sse",
      "headers": {
        "Authorization": "Bearer xoxb-demo-slack-token"
      }
    }
  }
}
JSON
echo "  Parent config: 3 inherited servers (github, jira, slack)"

# ── Project .mcp.json (locally selected servers) ────────────────────────────

cat > "$FIXTURE_DIR/workspace/my-project/.mcp.json" <<'JSON'
{
  "mcpServers": {
    "everything": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-everything"]
    },
    "filesystem": {
      "type": "stdio",
      "command": "false",
      "args": []
    },
    "kubernetes": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp"
    }
  }
}
JSON
echo "  Project config: 3 local servers (everything, filesystem, kubernetes)"

# ── User config (~/.claude.json) ────────────────────────────────────────────

cat > "$FIXTURE_DIR/home/.claude.json" <<'JSON'
{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Bearer ghp_demo_token_12345"
      }
    },
    "google-workspace": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp"
    },
    "jira": {
      "type": "http",
      "url": "http://127.0.0.1:1/mcp",
      "headers": {
        "Authorization": "Basic ZGVtbzpwYXNz"
      }
    },
    "slack": {
      "type": "sse",
      "url": "http://127.0.0.1:1/sse",
      "headers": {
        "Authorization": "Bearer xoxb-demo-slack-token"
      }
    }
  }
}
JSON
echo "  User config: 4 servers (github, google-workspace, jira, slack)"

# ── Plugin cache (~/.claude/plugins/cache/) ──────────────────────────────────
# Each plugin needs: <marketplace>/<name>/<version>/plugin.json

create_plugin() {
  local marketplace="$1" name="$2" version="$3" description="$4"
  local dir="$FIXTURE_DIR/home/.claude/plugins/cache/$marketplace/$name/$version"
  mkdir -p "$dir"
  cat > "$dir/plugin.json" <<JSON
{"description": "$description"}
JSON
}

create_plugin "cc-copyedit-marketplace" "copyedit" "1.5.0" \
  "Documentation copy editing and style checks"
create_plugin "cc-jira-marketplace" "jira" "2.0.1" \
  "JIRA issue tracking and project management"
create_plugin "cc-k8s-marketplace" "kubernetes" "0.9.0" \
  "Kubernetes manifest generation and diagnostics"
create_plugin "cc-prose-marketplace" "prose" "1.1.0" \
  "Technical writing assistant"
create_plugin "cc-sdd-marketplace" "sdd" "1.2.0" \
  "Specification-Driven Development workflows"
echo "  Plugin cache: 5 plugins"

# ── User plugin settings (~/.claude/settings.json) ──────────────────────────

cat > "$FIXTURE_DIR/home/.claude/settings.json" <<'JSON'
{
  "enabledPlugins": {
    "copyedit@cc-copyedit-marketplace": true,
    "jira@cc-jira-marketplace": true,
    "kubernetes@cc-k8s-marketplace": true,
    "prose@cc-prose-marketplace": true,
    "sdd@cc-sdd-marketplace": true
  }
}
JSON
echo "  User plugins: all 5 enabled"

# ── Project plugin settings (.claude/settings.json) ─────────────────────────
# Only stores the delta from user scope: prose disabled.

cat > "$FIXTURE_DIR/workspace/my-project/.claude/settings.json" <<'JSON'
{
  "enabledPlugins": {
    "prose@cc-prose-marketplace": false
  }
}
JSON
echo "  Project plugins: prose disabled"

echo ""
echo "Created demo fixtures in $FIXTURE_DIR"
echo ""
echo "Test manually with:"
echo "  HOME=$FIXTURE_DIR/home XDG_CONFIG_HOME=$FIXTURE_DIR/home/.config \\"
echo "    bash -c 'cd $FIXTURE_DIR/workspace/my-project && cc-setup'"
