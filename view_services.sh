#!/usr/bin/env zsh

SESSION="microservices"
SERVICES=("store" "payment" "delivery" "sales" "consumer1" "consumer2")

# Attach if session already exists
if tmux has-session -t "$SESSION" 2>/dev/null; then
  echo "Session '$SESSION' already exists. Attaching..."
  tmux attach-session -t "$SESSION"
  exit 0
fi

# Start container services in the background
docker compose up -d

# Create session with Pane 0 for the interactive TUI / control terminal
tmux new-session -d -s "$SESSION" -n "dashboard" "docker compose attach tui"

# Create a pane for each service log and organize into a grid
for service in "${SERVICES[@]}"; do
  tmux split-window -t "$SESSION:dashboard" "docker compose logs -f $service"
  tmux select-layout -t "$SESSION:dashboard" tiled
done

# Focus Pane 0 (Top-left) for your TUI
tmux select-pane -t "$SESSION:dashboard.0"

# (Optional) Automatically execute your TUI command here:
# tmux send-keys -t "$SESSION:dashboard.0" "your-tui-command" C-m

tmux attach-session -t "$SESSION"
