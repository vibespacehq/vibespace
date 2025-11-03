FROM ubuntu:24.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    vim \
    nano \
    build-essential \
    ca-certificates \
    openssh-client \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install code-server (latest 4.104.3+)
RUN curl -fsSL https://code-server.dev/install.sh | sh

# Install Node.js 24.x LTS (entering LTS October 2025)
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Upgrade npm to 11.6.2
RUN npm install -g npm@11.6.2

# Install Google Gemini CLI (Python-based)
# Using google-generativeai package (official Google Gemini SDK)
RUN pip3 install --break-system-packages google-generativeai

# Create workspace user (UID 1001 to avoid conflict with ubuntu user at 1000)
RUN useradd -m -s /bin/bash -u 1001 workspace && \
    mkdir -p /home/workspace/.local/share/code-server && \
    mkdir -p /home/workspace/.gemini && \
    chown -R workspace:workspace /home/workspace

USER workspace
WORKDIR /workspace

# Pre-configure code-server
RUN mkdir -p ~/.config/code-server && \
    echo "bind-addr: 0.0.0.0:8080" > ~/.config/code-server/config.yaml && \
    echo "auth: none" >> ~/.config/code-server/config.yaml && \
    echo "cert: false" >> ~/.config/code-server/config.yaml

# Apply custom workspaces theme
RUN mkdir -p ~/.local/share/code-server/User && \
    echo '{\n\
  "workbench.colorTheme": "Default Dark Modern",\n\
  "workbench.colorCustomizations": {\n\
    "activityBar.background": "#000000",\n\
    "activityBar.foreground": "#00ABAB",\n\
    "activityBar.activeBorder": "#F102F3",\n\
    "activityBar.inactiveForeground": "#666666",\n\
    "activityBar.border": "#1a1a1a",\n\
    "activityBarBadge.background": "#F102F3",\n\
    "activityBarBadge.foreground": "#ffffff",\n\
    "sideBar.background": "#0a0a0a",\n\
    "sideBar.foreground": "#ffffff",\n\
    "sideBar.border": "#1a1a1a",\n\
    "sideBarTitle.foreground": "#ffffff",\n\
    "sideBarSectionHeader.background": "#000000",\n\
    "sideBarSectionHeader.foreground": "#00ABAB",\n\
    "sideBarSectionHeader.border": "#1a1a1a",\n\
    "editor.background": "#000000",\n\
    "editor.foreground": "#ffffff",\n\
    "editorLineNumber.foreground": "#666666",\n\
    "editorLineNumber.activeForeground": "#00ABAB",\n\
    "editorCursor.foreground": "#00ABAB",\n\
    "editor.selectionBackground": "#00ABAB40",\n\
    "editor.inactiveSelectionBackground": "#00ABAB20",\n\
    "editor.findMatchBackground": "#F5F50A60",\n\
    "editor.findMatchHighlightBackground": "#F5F50A40",\n\
    "editorBracketMatch.background": "#00ABAB40",\n\
    "editorBracketMatch.border": "#00ABAB",\n\
    "editorGroupHeader.tabsBackground": "#0a0a0a",\n\
    "tab.activeBackground": "#000000",\n\
    "tab.activeForeground": "#ffffff",\n\
    "tab.activeBorder": "#00ABAB",\n\
    "tab.activeBorderTop": "#00ABAB",\n\
    "tab.inactiveBackground": "#0a0a0a",\n\
    "tab.inactiveForeground": "#a0a0a0",\n\
    "tab.border": "#1a1a1a",\n\
    "terminal.background": "#000000",\n\
    "terminal.foreground": "#ffffff",\n\
    "terminalCursor.foreground": "#00ABAB",\n\
    "terminalCursor.background": "#000000",\n\
    "terminal.selectionBackground": "#00ABAB40",\n\
    "terminal.border": "#1a1a1a",\n\
    "statusBar.background": "#F102F3",\n\
    "statusBar.foreground": "#ffffff",\n\
    "statusBar.noFolderBackground": "#000000",\n\
    "statusBar.debuggingBackground": "#FF7D4B",\n\
    "statusBarItem.hoverBackground": "#F5F50A",\n\
    "statusBarItem.hoverForeground": "#000000",\n\
    "button.background": "#F102F3",\n\
    "button.foreground": "#ffffff",\n\
    "button.hoverBackground": "#F5F50A",\n\
    "button.secondaryBackground": "#00ABAB",\n\
    "button.secondaryForeground": "#ffffff",\n\
    "button.secondaryHoverBackground": "#00d4d4",\n\
    "focusBorder": "#00ABAB",\n\
    "contrastBorder": "#1a1a1a",\n\
    "input.background": "#0a0a0a",\n\
    "input.border": "#1a1a1a",\n\
    "input.foreground": "#ffffff",\n\
    "input.placeholderForeground": "#666666",\n\
    "inputOption.activeBorder": "#00ABAB",\n\
    "inputValidation.errorBorder": "#f85149",\n\
    "inputValidation.warningBorder": "#d29922",\n\
    "list.activeSelectionBackground": "#00ABAB30",\n\
    "list.activeSelectionForeground": "#ffffff",\n\
    "list.hoverBackground": "#0a0a0a",\n\
    "list.focusBackground": "#00ABAB20",\n\
    "list.inactiveSelectionBackground": "#0a0a0a",\n\
    "list.highlightForeground": "#00ABAB",\n\
    "dropdown.background": "#0a0a0a",\n\
    "dropdown.border": "#1a1a1a",\n\
    "dropdown.foreground": "#ffffff",\n\
    "panel.background": "#000000",\n\
    "panel.border": "#1a1a1a",\n\
    "panelTitle.activeBorder": "#00ABAB",\n\
    "panelTitle.activeForeground": "#ffffff",\n\
    "panelTitle.inactiveForeground": "#a0a0a0",\n\
    "badge.background": "#F102F3",\n\
    "badge.foreground": "#ffffff",\n\
    "notificationCenter.border": "#1a1a1a",\n\
    "notifications.background": "#0a0a0a",\n\
    "notifications.border": "#1a1a1a",\n\
    "notifications.foreground": "#ffffff",\n\
    "notificationLink.foreground": "#00ABAB",\n\
    "progressBar.background": "#00ABAB",\n\
    "editorWidget.background": "#0a0a0a",\n\
    "editorWidget.border": "#1a1a1a",\n\
    "editorSuggestWidget.background": "#0a0a0a",\n\
    "editorSuggestWidget.selectedBackground": "#00ABAB30",\n\
    "editorSuggestWidget.highlightForeground": "#00ABAB",\n\
    "editorHoverWidget.background": "#0a0a0a",\n\
    "editorHoverWidget.border": "#1a1a1a",\n\
    "peekView.border": "#00ABAB",\n\
    "peekViewEditor.background": "#0a0a0a",\n\
    "peekViewResult.background": "#000000",\n\
    "peekViewTitle.background": "#0a0a0a",\n\
    "diffEditor.insertedTextBackground": "#3fb95030",\n\
    "diffEditor.removedTextBackground": "#f8514930"\n\
  },\n\
  "editor.fontFamily": "JetBrains Mono, monospace",\n\
  "editor.fontSize": 14,\n\
  "editor.lineHeight": 1.6,\n\
  "editor.fontLigatures": true,\n\
  "terminal.integrated.fontFamily": "JetBrains Mono, monospace",\n\
  "terminal.integrated.fontSize": 14,\n\
  "telemetry.telemetryLevel": "off",\n\
  "redhat.telemetry.enabled": false,\n\
  "workbench.enableExperiments": false\n\
}' > ~/.local/share/code-server/User/settings.json

# Copy base AGENT.md
COPY AGENT.md /home/workspace/.gemini/AGENT.md

# Agent initialization script
RUN echo '#!/bin/bash\n\
echo "Workspace Environment:"\n\
echo "  Node.js: $(node --version)"\n\
echo "  npm: $(npm --version)"\n\
echo "  Python: $(python3 --version)"\n\
echo "  code-server: $(code-server --version | head -1)"\n\
if [ -n "$GOOGLE_API_KEY" ]; then\n\
  echo "  ✓ Google Gemini API available"\n\
fi' > /home/workspace/init-agents.sh && chmod +x /home/workspace/init-agents.sh

EXPOSE 8080

CMD ["/bin/bash", "-c", "~/init-agents.sh && exec code-server /workspace"]
