#!/usr/bin/env zsh

SESSION="microservices"

# Attach if session already exists
if tmux has-session -t "$SESSION" 2>/dev/null; then
  echo "Session '$SESSION' already exists. Attaching..."
  tmux attach-session -t "$SESSION"
  exit 0
fi

# Start container services in the background
docker compose up -d

# 1. Row 1 (Top 50% height): TUI spanning full width (Pane 0)
tmux new-session -d -s "$SESSION" -n "dashboard" "docker compose attach tui"

# 2. Row 2 (Middle 25% height): First log row start (Pane 1)
tmux split-window -h  -t "$SESSION:dashboard.0" "docker compose logs -f store"

# 3. Row 3 (Bottom 25% height): Second log row start (Pane 2)
tmux split-window -v  -t "$SESSION:dashboard.1" "docker compose logs -f sales"

# 4. Split Middle Row into 3 equal columns: [ store | payment | delivery ]
tmux split-window -v  -t "$SESSION:dashboard.1" "docker compose logs -f payment"
tmux split-window -h  -t "$SESSION:dashboard.3" "docker compose logs -f delivery"

# 5. Split Bottom Row into 3 equal columns: [ sales | consumer1 | consumer2 ]
tmux split-window -h  -t "$SESSION:dashboard.2" "docker compose logs -f consumer1"
tmux split-window -h  -t "$SESSION:dashboard.5" "docker compose logs -f consumer2"

# Focus Pane 0 (Top TUI pane)
tmux select-pane -t "$SESSION:dashboard.0"

tmux attach-session -t "$SESSION"
