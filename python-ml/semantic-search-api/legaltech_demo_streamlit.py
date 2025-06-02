import streamlit as st
import requests
import pandas as pd
import re
import redis
import json
import os
import time

API_URL = "http://semantic-api:8000/api"

st.set_page_config(page_title="LegalTech Semantic Search Demo", layout="wide")
st.title("⚖️ LegalTech Semantic Search API – Demo")

st.markdown("""
<style>
.result-card {background: #fff; border-radius: 10px; box-shadow: 0 2px 8px #eee; margin-bottom: 1rem; padding: 1rem;}
.suggestion {background: #e3f2fd; color: #1976d2; padding: 0.2rem 0.5rem; border-radius: 3px; margin-right: 0.3rem; cursor:pointer; display:inline-block;}
.smart-btn {background: #4CAF50; color: white; border: none; border-radius: 5px; padding: 0.3rem 0.7rem; margin-right: 0.5rem; cursor:pointer;}
</style>
""", unsafe_allow_html=True)

# --- Sidebar: Example data ---
if st.sidebar.button("Données de démo (sandbox)"):
    st.session_state["docs"] = [
        "Contrat de prestation de services signé le 12/01/2024 entre AcmeCorp et BetaInc.",
        "Facture n°2024-001 émise le 15/01/2024 pour un montant de 5000€.",
        "Email RH : Demande de congé pour la période du 10/02/2024 au 20/02/2024.",
        "Arrêt Cour de cassation 2023 : clause abusive constatée.",
        "Confidentiality clause must be signed by both parties."
    ]
    st.session_state["metadatas"] = [
        {"type": "contrat", "lang": "fr", "client": "AcmeCorp", "date": "2024-01-12"},
        {"type": "facture", "lang": "fr", "client": "AcmeCorp", "date": "2024-01-15"},
        {"type": "email", "lang": "fr", "client": "AcmeCorp", "date": "2024-02-10"},
        {"type": "jurisprudence", "lang": "fr", "jurisdiction": "cassation", "date": "2023-11-05"},
        {"type": "contract", "lang": "en", "client": "BetaInc", "date": "2024-01-15"}
    ]

# --- Document upload/input ---
st.header("1️⃣ Indexer des documents juridiques")
docs = st.text_area("Collez vos documents (un par ligne)", value="\n".join(st.session_state.get("docs", [])), height=120)
metadatas = st.text_area("Collez les métadonnées JSON (une par ligne, optionnel)", value="\n".join([str(m) for m in st.session_state.get("metadatas", [])]), height=120)

if st.button("Indexer"):
    docs_list = [d.strip() for d in docs.split("\n") if d.strip()]
    try:
        metadatas_list = [eval(m) for m in metadatas.split("\n") if m.strip()]
    except Exception:
        metadatas_list = [{} for _ in docs_list]
    payload = {"documents": docs_list, "metadata": metadatas_list}
    r = requests.post(f"{API_URL}/index", json=payload)
    if r.status_code == 200:
        st.success(f"{r.json()['indexed']} documents indexés !")
    else:
        st.error(f"Erreur : {r.text}")

# --- Recherche sémantique avec auto-complete ---
st.header("2️⃣ Recherche sémantique contextuelle")
col1, col2 = st.columns(2)
with col1:
    if "query" not in st.session_state:
        st.session_state["query"] = ""
    query = st.text_input("Votre requête (ex: clause de non-concurrence)", st.session_state["query"])
    # Auto-complete
    if query:
        try:
            ac = requests.get(f"{API_URL}/autocomplete", params={"prefix": query}).json()
            suggestions = ac.get("suggestions", [])
        except Exception:
            suggestions = []
        if suggestions:
            st.markdown("Suggestions :", unsafe_allow_html=True)
            for s in suggestions:
                if st.button(s, key=f"sugg_{s}"):
                    st.session_state["query"] = s
                    st.experimental_rerun()
    top_k = st.slider("Nombre de résultats", 1, 20, 5)
with col2:
    st.markdown("**Filtres avancés (optionnel)**")
    type_filter = st.text_input("Type (contrat, jurisprudence, email...)")
    client_filter = st.text_input("Client")
    lang_filter = st.text_input("Langue (fr, en...)")
    date_filter = st.text_input("Date (ex: 2024-01-10 ou plage)")
filters = {}
if type_filter: filters["type"] = type_filter
if client_filter: filters["client"] = client_filter
if lang_filter: filters["lang"] = lang_filter
if date_filter: filters["date"] = date_filter

if st.button("Rechercher"):
    payload = {"query": st.session_state["query"], "top_k": top_k, "filters": filters}
    r = requests.post(f"{API_URL}/search", json=payload)
    if r.status_code == 200:
        results = r.json()["results"]
        if results:
            df = pd.DataFrame(results)
            st.download_button("Exporter CSV", df.to_csv(index=False).encode(), "resultats.csv", "text/csv")
            for i, res in enumerate(results):
                st.markdown(f'<div class="result-card">', unsafe_allow_html=True)
                st.write(f"**Score**: {res['score']:.2f}")
                st.write(f"**Texte**: {res['text']}")
                # Résumé IA avancé
                if st.button("Résumé IA", key=f"summ_{i}"):
                    with st.spinner("Génération du résumé..."):
                        payload = {
                            "text": res["text"],
                            "engine": st.session_state["summarizer_engine"],
                            "openai_api_key": st.session_state["openai_api_key"]
                        }
                        rsum = requests.post(f"{API_URL}/summarize", json=payload).json()
                        if "summary" in rsum:
                            st.success(rsum["summary"])
                        else:
                            st.error(rsum.get("error", "Erreur inconnue"))
                # Résumé automatique (première phrase)
                summary = res['text'].split('.')[0] + '.' if '.' in res['text'] else res['text']
                st.write(f"**Résumé**: {summary}")
                # Extraction d'entités (dates, montants, parties)
                dates = re.findall(r'\d{4}-\d{2}-\d{2}', res['text'])
                parties = re.findall(r'\b[A-Z][a-zA-Z]+(?: [A-Z][a-zA-Z]+)*\b', res['text'])
                st.write(f"**Dates**: {dates}")
                st.write(f"**Parties**: {parties}")
                # Smart Copy
                if st.button("Smart Copy", key=f"copy_{i}"):
                    st.toast("Texte copié dans le presse-papier (simulé)")
                # Find Similar interactif (simple ou grouped)
                find_mode = st.selectbox("Mode Find Similar", ["simple", "grouped"], key=f"findmode_{i}", format_func=lambda x: "Top similaires" if x=="simple" else "Groupes de variantes")
                if st.button("Find Similar", key=f"sim_{i}"):
                    with st.spinner("Recherche des documents similaires..."):
                        payload = {
                            "doc_id": res["id"],
                            "top_k": 5,
                            "mode": find_mode
                        }
                        simres = requests.post(f"{API_URL}/find_similar", json=payload).json()
                        if "error" in simres:
                            st.error(simres["error"])
                        elif find_mode == "simple":
                            st.write("**Top similaires :**")
                            for r in simres["results"]:
                                st.write(f"- {r['text']} (score: {r['score']:.2f})")
                        else:
                            st.write("**Groupes de variantes :**")
                            for g in simres.get("groups", []):
                                st.write(", ".join([f"{r['text']} (score: {r['score']:.2f})" for r in g]))
                # Extraction d'entités avancée
                if st.button("Extraction d'entités", key=f"ent_{i}"):
                    with st.spinner("Extraction en cours..."):
                        payload = {
                            "text": res["text"],
                            "engine": st.session_state["entities_engine"],
                            "lang": st.session_state["entities_lang"]
                        }
                        ents = requests.post(f"{API_URL}/entities", json=payload).json()
                        st.info(f"Entités extraites : {ents}")
                st.markdown('</div>', unsafe_allow_html=True)
        else:
            st.info("Aucun résultat trouvé.")
    else:
        st.error(f"Erreur : {r.text}")

# --- Stats ---
st.header("3️⃣ Statistiques de l'index")
if st.button("Voir les stats"):
    r = requests.get(f"{API_URL}/stats")
    if r.status_code == 200:
        st.json(r.json())
    else:
        st.error(f"Erreur : {r.text}")

# --- Dashboard Analytics ---
st.header("4️⃣ Dashboard Analytics & ROI")
if st.button("Afficher le dashboard analytics"):
    with st.spinner("Chargement des stats..."):
        stats = requests.get(f"{API_URL}/analytics").json()
        st.metric("Nombre de recherches", stats["searches"])
        st.metric("Nombre d'indexations", stats["indexations"])
        st.metric("Temps de réponse moyen (ms)", stats["avg_response_time_ms"])
        st.metric("Dernière recherche (ms)", stats["last_search_time_ms"])
        # Top requêtes
        st.subheader("Top requêtes")
        if stats["top_queries"]:
            dfq = pd.DataFrame(stats["top_queries"], columns=["Requête", "Nombre"])
            st.bar_chart(dfq.set_index("Requête"))
            st.download_button("Exporter top requêtes", dfq.to_csv(index=False).encode(), "top_requetes.csv", "text/csv")
        # Top documents
        st.subheader("Top documents")
        if stats["top_docs"]:
            dfd = pd.DataFrame(stats["top_docs"], columns=["DocID", "Nombre"])
            st.bar_chart(dfd.set_index("DocID"))
            st.download_button("Exporter top docs", dfd.to_csv(index=False).encode(), "top_docs.csv", "text/csv")

# --- Agents AI avancés ---
st.header("6️⃣ Agents AI avancés")
agent_type = st.selectbox("Choisir un agent AI", ["Audit Express local", "Audit LLM", "Veille automatisée"])
engine = st.selectbox("Moteur", ["local", "openai"], format_func=lambda x: "Local (rapide, privacy)" if x=="local" else "OpenAI (LLM, cloud)")
openai_api_key = ""
if engine == "openai":
    openai_api_key = st.text_input("OpenAI API Key", type="password")

if agent_type == "Audit Express local":
    min_years = st.number_input("Durée seuil (ans)", min_value=1, max_value=10, value=2)
    if st.button("Lancer l'audit local"):
        with st.spinner("Audit en cours..."):
            rapport = requests.get(f"{API_URL}/audit", params={"min_years": min_years}).json()
            st.metric("Total documents", rapport["total_docs"])
            st.metric("Clauses non-concurrence non conformes", rapport["non_concurrence_trouvees"])
            st.write(rapport["recommandation"])
            if rapport["docs_non_conformes"]:
                for d in rapport["docs_non_conformes"]:
                    st.write(f"- {d['text']} (Durée: {d['duree']} ans, ID: {d['id']})")

elif agent_type == "Audit LLM":
    min_years = st.number_input("Durée seuil (ans)", min_value=1, max_value=10, value=2, key="llm_min_years")
    if st.button("Lancer l'audit LLM"):
        with st.spinner("Audit LLM en cours..."):
            payload = {"engine": engine, "openai_api_key": openai_api_key, "min_years": min_years}
            rapport = requests.post(f"{API_URL}/audit_llm", json=payload).json()
            if "error" in rapport:
                st.error(rapport["error"])
            else:
                st.metric("Total documents", rapport.get("total_docs", 0))
                st.metric("Clauses non-concurrence non conformes", rapport.get("non_concurrence_trouvees", rapport.get("flagged", 0)))
                if "recommandation" in rapport:
                    st.write(rapport["recommandation"])
                if rapport.get("docs_non_conformes"):
                    for d in rapport["docs_non_conformes"]:
                        st.write(f"- {d['text']} (Durée: {d['duree']} ans, ID: {d['id']})")
                if rapport.get("docs_flagged"):
                    for d in rapport["docs_flagged"]:
                        st.write(f"- {d['text']} (LLM: {d['llm']})")

elif agent_type == "Veille automatisée":
    keyword = st.text_input("Mot-clé de veille", value="jurisprudence")
    if st.button("Lancer la veille"):
        with st.spinner("Veille en cours..."):
            payload = {"engine": engine, "openai_api_key": openai_api_key, "keyword": keyword}
            res = requests.post(f"{API_URL}/veille", json=payload).json()
            if "error" in res:
                st.error(res["error"])
            else:
                st.metric("Total documents", res.get("total_docs", 0))
                st.metric("Alertes détectées", res.get("alerts", 0))
                if res.get("docs_alerts"):
                    for d in res["docs_alerts"]:
                        st.write(f"- {d['text']} (ID: {d['id']})")

# Ajout : choix du moteur de résumé IA et clé OpenAI
st.sidebar.header("Résumé IA avancé")
st.session_state.setdefault("summarizer_engine", "hf")
st.session_state.setdefault("openai_api_key", "")
st.session_state["summarizer_engine"] = st.sidebar.selectbox("Moteur de résumé IA", ["hf", "openai"], format_func=lambda x: "HuggingFace (local)" if x=="hf" else "OpenAI (cloud)", index=0 if st.session_state["summarizer_engine"]=="hf" else 1)
if st.session_state["summarizer_engine"] == "openai":
    st.session_state["openai_api_key"] = st.sidebar.text_input("OpenAI API Key", type="password", value=st.session_state["openai_api_key"])

# Ajout : choix du moteur d'extraction d'entités et langue
st.sidebar.header("Extraction d'entités avancée")
st.session_state.setdefault("entities_engine", "spacy")
st.session_state.setdefault("entities_lang", "fr")
st.session_state["entities_engine"] = st.sidebar.selectbox("Moteur d'extraction", ["spacy", "regex"], format_func=lambda x: "spaCy (avancé)" if x=="spacy" else "Regex (rapide)", index=0 if st.session_state["entities_engine"]=="spacy" else 1)
st.session_state["entities_lang"] = st.sidebar.selectbox("Langue", ["fr", "en"], index=0 if st.session_state["entities_lang"]=="fr" else 1)

if "Monitoring" not in st.session_state:
    st.session_state["Monitoring"] = {}

menu = st.sidebar.radio("Menu", ["Recherche", "Indexation", "Analytics", "Monitoring"])

if menu == "Monitoring":
    st.header("Monitoring du cache Redis & recherches")
    try:
        r = redis.Redis(host=os.environ.get("REDIS_HOST", "localhost"), port=6379, db=0, decode_responses=True)
        keys = r.keys("search:*")
        cache_size = sum([r.memory_usage(k) or 0 for k in keys])
        last_keys = keys[-5:]
        last_queries = []
        for k in last_keys:
            val = r.get(k)
            if val:
                try:
                    last_queries.append(json.loads(val).get("query", "?"))
                except Exception:
                    last_queries.append("?")
        st.metric("Clés de cache (search:*)", len(keys))
        st.metric("Taille totale du cache (KB)", round(cache_size/1024, 2))
        st.write("**5 dernières requêtes**:")
        st.write(last_queries)
    except Exception as e:
        st.error(f"Erreur Redis: {e}")
    # Analytics
    try:
        resp = requests.get("http://localhost:8000/api/analytics", headers={"x-api-key": "test-key", "x-role": "admin"})
        if resp.status_code == 200:
            data = resp.json()
            st.metric("Nombre de recherches", data.get("searches", 0))
            # Taux de cache hit (estimation naïve)
            hits = len([k for k in keys if k])
            misses = data.get("searches", 0) - hits
            taux = hits/(hits+misses) if (hits+misses) > 0 else 0
            st.metric("Taux de cache hit (est.)", f"{taux*100:.1f}%")
        else:
            st.warning("Analytics non disponibles")
    except Exception as e:
        st.warning(f"Analytics: {e}")
    st.caption("Rafraîchissement auto toutes les 5s")
    st.experimental_rerun()
    time.sleep(5) 