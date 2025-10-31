ARG AGENT=claude
FROM registry.default.svc.cluster.local:5000/workspace-base-${AGENT}:latest

USER root

# Install Python 3.12 (Ubuntu 24.04 default) and pip
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    python3-dev \
    python3-venv \
    && rm -rf /var/lib/apt/lists/* \
    && rm -f /usr/lib/python3.12/EXTERNALLY-MANAGED

USER coder

# Add user bin to PATH for pip-installed executables
ENV PATH="/home/coder/.local/bin:${PATH}"

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
COPY CLAUDE.md /home/coder/JUPYTER-GUIDE.md
RUN mkdir -p ~/.claude ~/.openai ~/.gemini && \
    cp /home/coder/JUPYTER-GUIDE.md ~/.claude/JUPYTER-GUIDE.md 2>/dev/null || true && \
    cp /home/coder/JUPYTER-GUIDE.md ~/.openai/JUPYTER-GUIDE.md 2>/dev/null || true && \
    cp /home/coder/JUPYTER-GUIDE.md ~/.gemini/JUPYTER-GUIDE.md 2>/dev/null || true

# Configure Jupyter Lab
RUN mkdir -p ~/.jupyter && \
    ~/.local/bin/jupyter lab --generate-config && \
    echo "c.ServerApp.ip = '0.0.0.0'" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.port = 8888" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.token = ''" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.password = ''" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.allow_root = False" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.open_browser = False" >> ~/.jupyter/jupyter_lab_config.py

# Create init script with version display
RUN echo '#!/bin/bash\n\
# Ensure PATH includes user bin for pip-installed executables\n\
export PATH="/home/coder/.local/bin:$PATH"\n\
echo "Starting Jupyter Lab..."\n\
jupyter lab --no-browser &>/dev/null &\n\
sleep 2\n\
echo "✓ Jupyter Lab running on port 8888"\n\
echo ""\n\
echo "Installed versions:"\n\
echo "  Python: $(python3 --version)"\n\
echo "  Jupyter Lab: $(jupyter lab --version)"\n\
echo "  NumPy: $(pip3 show numpy | grep Version | cut -d\" \" -f2)"\n\
echo "  Pandas: $(pip3 show pandas | grep Version | cut -d\" \" -f2)"\n\
echo "  Matplotlib: $(pip3 show matplotlib | grep Version | cut -d\" \" -f2)"\n\
~/init-agents.sh\n\
exec code-server /workspace' > /home/coder/init.sh && \
    chmod +x /home/coder/init.sh

EXPOSE 8080 8888

CMD ["/home/coder/init.sh"]
