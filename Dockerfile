FROM docker/sandbox-templates:claude-code

USER root

# Install system utilities
RUN apt-get update && apt-get install -y --no-install-recommends \
    jq \
    tree \
    make \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install pnpm
RUN npm install -g pnpm@latest

# Install Playwright dependencies for Chromium
RUN npx playwright install-deps chromium

USER agent

# Pre-install Playwright browsers (optional, makes first run faster)
RUN npx playwright install chromium
