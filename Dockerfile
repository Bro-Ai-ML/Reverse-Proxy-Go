# Utilise une image officielle Python ARM64 compatible M1
FROM --platform=linux/arm64 python:3.10-slim

# Crée un utilisateur non-root
RUN useradd -ms /bin/bash dsuser

# Installe les dépendances système nécessaires
RUN apt-get update && apt-get install -y \
    build-essential \
    git \
    curl \
    cmake \
    libopenmpi-dev \
    python3-dev \
    gfortran \
    libatlas-base-dev \
    libglib2.0-0 \
    libsm6 \
    libxrender1 \
    libxext6 \
    && rm -rf /var/lib/apt/lists/*

# Passe à l'utilisateur non-root
USER dsuser
WORKDIR /workspace

# Ajoute le chemin des binaires pip user au PATH
ENV PATH="/home/dsuser/.local/bin:$PATH"

# Copie le requirements.txt (sera créé ensuite)
COPY --chown=dsuser:dsuser requirements.txt ./

# Upgrade pip et outils de build
RUN pip install --upgrade pip setuptools wheel

# Installe les dépendances Python
RUN pip install --no-cache-dir -r requirements.txt

# Expose le port Jupyter
EXPOSE 8888

# Commande de démarrage par défaut : Jupyter Lab
CMD ["jupyter-lab", "--ip=0.0.0.0", "--port=8888", "--no-browser", "--allow-root", "--NotebookApp.token=''"] 