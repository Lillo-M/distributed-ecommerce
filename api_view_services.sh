#!/bin/bash

SESSION="dashboard"

# Start container services in the background
docker compose up -d

# 1. Row 1 (Top 50% height): TUI spanning full width (Pane 0)
tmux new-session -d -s "$SESSION" -n "dashboard" "docker compose attach api"

# 2. Split vertically to create the bottom half (Pane 1)
tmux split-window -v -p 50 -t "$SESSION:dashboard" "docker compose logs -f store"

# 3. Split the bottom half vertically again to create Row 2 and Row 3 (Pane 1 & Pane 2)
tmux split-window -v -p 50 -t "$SESSION:dashboard.1" "docker compose logs -f sales"

# --- Current Layout ---
# Pane 0: api (Top 50%)
# Pane 1: store (Middle 25%)
# Pane 2: sales (Bottom 25%)

# 4. Split Middle Row (Pane 1) into 3 equal columns: [ store | payment | delivery ]
# Take 66% of Pane 1 for the right side (becomes Pane 2)
tmux split-window -h -p 66 -t "$SESSION:dashboard.1" "docker compose logs -f payment"
# Take 50% of the new Pane 2 for the final third (becomes Pane 3)
tmux split-window -h -p 50 -t "$SESSION:dashboard.2" "docker compose logs -f delivery"

# --- Current Layout ---
# Pane 0: api
# Pane 1: store, Pane 2: payment, Pane 3: delivery
# Pane 4: sales (Pushed down from Pane 2 to Pane 4)

# 5. Split Bottom Row (Pane 4) into 3 equal columns: [ sales | consumer1 | consumer2 ]
# Take 66% of Pane 4 for the right side (becomes Pane 5)
tmux split-window -h -p 66 -t "$SESSION:dashboard.4" "docker compose logs -f consumer1"
# Take 50% of the new Pane 5 for the final third (becomes Pane 6)
tmux split-window -h -p 50 -t "$SESSION:dashboard.5" "docker compose logs -f consumer2"

# Focus Pane 0 (Top TUI pane)
tmux select-pane -t "$SESSION:dashboard.0"

# Attach to the session
tmux attach-session -t "$SESSION"
