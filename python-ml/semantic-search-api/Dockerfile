FROM --platform=linux/arm64 python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Installer streamlit et dépendances pour la démo
RUN pip install --no-cache-dir streamlit requests pandas

# Télécharger le modèle au build
# RUN python -c "from sentence_transformers import SentenceTransformer; SentenceTransformer('all-MiniLM-L6-v2')"

RUN pip install prometheus-fastapi-instrumentator

COPY . .

EXPOSE 8000
EXPOSE 8501

# Entrypoint dynamique : API ou Streamlit
ENV MODE=api
CMD ["/bin/sh", "-c", "if [ $MODE = 'streamlit' ]; then streamlit run legaltech_demo_streamlit.py --server.port=8501 --server.address=0.0.0.0; else uvicorn app:app --host 0.0.0.0 --port 8000; fi"] 