ARG AGENT=claude
FROM registry.default.svc.cluster.local:5000/vibespace-base-${AGENT}:latest

USER root

# Install Python 3.12 (Ubuntu 24.04 default) and pip
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    python3-dev \
    python3-venv \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /usr/lib/python3.12/EXTERNALLY-MANAGED

USER workspace

# Add user bin to PATH for pip-installed executables
ENV PATH="/home/workspace/.local/bin:${PATH}"

# Install Jupyter Lab 4.4.9 and data science libraries (latest)
RUN pip3 install --user --no-cache-dir \
    jupyterlab==4.4.9 \
    numpy \
    pandas \
    matplotlib \
    seaborn \
    scikit-learn \
    scipy \
    plotly \
    ipywidgets \
    notebook

# Install Python VS Code extensions (non-fatal - continue even if some extensions fail)
RUN (code-server --install-extension ms-python.python || true) && \
    (code-server --install-extension ms-toolsai.jupyter || true)

# Copy Jupyter-specific agent instructions
# Copy to all agent directories (only the relevant one will exist based on base image)
COPY CLAUDE.md /home/workspace/JUPYTER-GUIDE.md
RUN mkdir -p ~/.claude ~/.openai ~/.gemini && \
    cp /home/workspace/JUPYTER-GUIDE.md ~/.claude/JUPYTER-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/JUPYTER-GUIDE.md ~/.openai/JUPYTER-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/JUPYTER-GUIDE.md ~/.gemini/JUPYTER-GUIDE.md 2>/dev/null || true

# Configure Jupyter Lab
RUN mkdir -p ~/.jupyter && \
    ~/.local/bin/jupyter lab --generate-config && \
    echo "c.ServerApp.ip = '0.0.0.0'" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.port = 8888" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.token = ''" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.password = ''" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.allow_root = False" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.open_browser = False" >> ~/.jupyter/jupyter_lab_config.py

# Create init script with version display and project structure
RUN echo '#!/bin/bash\n\
# Ensure PATH includes user bin for pip-installed executables\n\
export PATH="/home/workspace/.local/bin:$PATH"\n\
\n\
# Initialize workspace structure on first run\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Setting up Jupyter workspace..."\n\
  cd /workspace\n\
  \n\
  # Create project structure\n\
  mkdir -p notebooks data/raw data/processed scripts outputs\n\
  \n\
  # Copy guide to workspace\n\
  cp ~/JUPYTER-GUIDE.md ./README.md\n\
  \n\
  # Create example notebook\n\
  cat > notebooks/01-getting-started.ipynb << '\''"'"'NOTEBOOK'\''"'"'\n\
{\n\
 "cells": [\n\
  {\n\
   "cell_type": "markdown",\n\
   "metadata": {},\n\
   "source": [\n\
    "# Getting Started with Jupyter Lab\\n",\n\
    "\\n",\n\
    "Welcome to your Jupyter workspace! This notebook will help you get started.\\n",\n\
    "\\n",\n\
    "## Quick Start\\n",\n\
    "\\n",\n\
    "1. Run cells with **Shift+Enter**\\n",\n\
    "2. Add new cells with **B** (below) or **A** (above)\\n",\n\
    "3. Change cell type with **M** (markdown) or **Y** (code)\\n",\n\
    "4. See all shortcuts with **H**"\n\
   ]\n\
  },\n\
  {\n\
   "cell_type": "code",\n\
   "execution_count": null,\n\
   "metadata": {},\n\
   "source": [\n\
    "# Import common libraries\\n",\n\
    "import numpy as np\\n",\n\
    "import pandas as pd\\n",\n\
    "import matplotlib.pyplot as plt\\n",\n\
    "import seaborn as sns\\n",\n\
    "\\n",\n\
    "print(\\"Libraries loaded successfully!\\")"\n\
   ],\n\
   "outputs": []\n\
  },\n\
  {\n\
   "cell_type": "markdown",\n\
   "metadata": {},\n\
   "source": [\n\
    "## Project Structure\\n",\n\
    "\\n",\n\
    "```\\n",\n\
    "notebooks/     - Your Jupyter notebooks\\n",\n\
    "data/raw/      - Original datasets\\n",\n\
    "data/processed/- Cleaned data\\n",\n\
    "scripts/       - Python scripts\\n",\n\
    "outputs/       - Plots and reports\\n",\n\
    "```"\n\
   ]\n\
  }\n\
 ],\n\
 "metadata": {\n\
  "kernelspec": {\n\
   "display_name": "Python 3",\n\
   "language": "python",\n\
   "name": "python3"\n\
  },\n\
  "language_info": {\n\
   "name": "python",\n\
   "version": "3.12.3"\n\
  }\n\
 },\n\
 "nbformat": 4,\n\
 "nbformat_minor": 4\n\
}\n\
NOTEBOOK\n\
  \n\
  # Create .gitignore\n\
  cat > .gitignore << '\''"'"'GITIGNORE'\''"'"'\n\
# Jupyter\n\
.ipynb_checkpoints/\n\
*/.ipynb_checkpoints/*\n\
\n\
# Python\n\
__pycache__/\n\
*.py[cod]\n\
*$py.class\n\
*.so\n\
.Python\n\
\n\
# Data\n\
data/raw/*\n\
!data/raw/.gitkeep\n\
\n\
# Outputs\n\
outputs/*\n\
!outputs/.gitkeep\n\
GITIGNORE\n\
  \n\
  # Create .gitkeep files\n\
  touch data/raw/.gitkeep data/processed/.gitkeep scripts/.gitkeep outputs/.gitkeep\n\
  \n\
  touch /workspace/.initialized\n\
  echo "✓ Workspace structure created"\n\
  echo ""\n\
fi\n\
\n\
echo "Starting Jupyter Lab..."\n\
jupyter lab --no-browser &>/dev/null &\n\
sleep 2\n\
echo "✓ Jupyter Lab running on port 8888"\n\
echo ""\n\
echo "Installed versions:"\n\
echo "  Python: $(python3 --version)"\n\
echo "  Jupyter Lab: $(jupyter lab --version)"\n\
echo "  NumPy: $(pip3 show numpy | grep Version | cut -d\\\" \\\" -f2)"\n\
echo "  Pandas: $(pip3 show pandas | grep Version | cut -d\\\" \\\" -f2)"\n\
echo "  Matplotlib: $(pip3 show matplotlib | grep Version | cut -d\\\" \\\" -f2)"\n\
~/init-agents.sh\n\
exec code-server /workspace' > /home/workspace/init.sh && \
    chmod +x /home/workspace/init.sh

EXPOSE 8080 8888

CMD ["/home/workspace/init.sh"]
