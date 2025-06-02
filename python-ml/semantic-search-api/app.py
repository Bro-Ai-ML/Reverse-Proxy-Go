from fastapi import FastAPI, HTTPException, Request, Body, Query
from fastapi.middleware.cors import CORSMiddleware
from models import SearchRequest, SearchResponse, IndexRequest
from semantic_engine import SemanticSearchEngine
from typing import Optional, List, Dict
from pydantic import BaseModel, validator, Field
import os
from collections import defaultdict
import time
from fastapi.responses import JSONResponse, StreamingResponse
import json
from datetime import datetime
from functools import wraps
from fastapi import status
import re
import io
from prometheus_fastapi_instrumentator import Instrumentator
import requests as pyrequests
import logging
import sys
from slowapi import Limiter
from slowapi.util import get_remote_address
import stripe
import redis as redislib
import secrets
from explainability import SearchExplainer, BasicTokenizer

# Ajout pour résumé IA
# try:
#     from transformers import pipeline
#     summarizer = pipeline("summarization", model="facebook/bart-large-cnn")
# except Exception:
#     summarizer = None

# Extraction d'entités avancée
try:
    import spacy
    nlp_fr = spacy.blank("fr")
    nlp_en = spacy.blank("en")
    try:
        nlp_fr = spacy.load("fr_core_news_md")
    except Exception:
        pass
    try:
        nlp_en = spacy.load("en_core_web_md")
    except Exception:
        pass
except Exception:
    nlp_fr = None
    nlp_en = None

app = FastAPI(title="Semantic Search API", version="1.0")
app.add_middleware(CORSMiddleware, allow_origins=["*"])

# Instrumentation Prometheus
Instrumentator().instrument(app).expose(app, endpoint="/metrics")

# --- API key management ---
API_KEYS = {
    os.getenv('ADMIN_KEY', 'admin-key'): 'admin',
    os.getenv('USER_KEY', 'user-key'): 'user'
}

@app.middleware("http")
async def verify_api_key(request: Request, call_next):
    api_key = request.headers.get("x-api-key")
    if api_key not in API_KEYS:
        return JSONResponse(status_code=401, content={"error": "Invalid API key"})
    request.state.role = API_KEYS[api_key]
    response = await call_next(request)
    return response

# --- Role enforcement decorator ---
def require_role(role_required):
    def decorator(func):
        @wraps(func)
        async def wrapper(*args, **kwargs):
            from fastapi import Request
            request = kwargs.get('request')
            if not request:
                for arg in args:
                    if isinstance(arg, Request):
                        request = arg
                        break
            user_role = getattr(request.state, 'role', None)
            if user_role != role_required:
                return JSONResponse(status_code=403, content={"detail": f"Role '{role_required}' required"})
            return await func(*args, **kwargs)
        return wrapper
    return decorator

# --- Health check endpoint ---
@app.get("/health")
async def health():
    redis_ok = False
    chroma_ok = False
    try:
        redis_ok = redis_store.ping()
    except Exception:
        pass
    try:
        chroma_ok = hasattr(search_engine.storage, 'chroma_client')
    except Exception:
        pass
    return {
        "status": "ok",
        "services": {
            "redis": redis_ok,
            "chroma": chroma_ok,
            "faiss": True
        }
    }

search_engine = SemanticSearchEngine()

# Analytics en mémoire (MVP)
analytics = {
    "searches": 0,
    "indexations": 0,
    "top_queries": defaultdict(int),
    "top_docs": defaultdict(int),
    "response_times": [],
    "last_search_time": 0.0
}

# --- Redis-based usage and plan tracking ---
REDIS_HOST = os.environ.get("REDIS_HOST", "redis")
REDIS_PORT = int(os.environ.get("REDIS_PORT", 6379))
redis_store = redislib.Redis(host=REDIS_HOST, port=REDIS_PORT, db=0, decode_responses=True)

# --- Configurable plan mapping ---
PLANS = {
    "basic": {"max_calls": 1000},
    "pro": {"max_calls": 10000},
    "enterprise": {"max_calls": 1000000}
}
STRIPE_PLAN_MAP = {
    "price_basic": "basic",
    "price_pro": "pro",
    "price_enterprise": "enterprise"
}

# --- Require Stripe secrets at startup ---
STRIPE_SECRET_KEY = os.getenv("STRIPE_SECRET_KEY")
STRIPE_WEBHOOK_SECRET = os.getenv("STRIPE_WEBHOOK_SECRET")
if not STRIPE_SECRET_KEY or not STRIPE_WEBHOOK_SECRET:
    raise RuntimeError("STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET must be set in environment.")
stripe.api_key = STRIPE_SECRET_KEY

from slowapi.errors import RateLimitExceeded
from slowapi.util import get_remote_address
from slowapi import Limiter
limiter = Limiter(key_func=get_remote_address)
app.state.limiter = limiter

@app.post("/api/stripe_webhook")
@limiter.limit("5/minute")
async def stripe_webhook(request: Request):
    payload = await request.body()
    sig_header = request.headers.get("stripe-signature")
    try:
        event = stripe.Webhook.construct_event(payload, sig_header, STRIPE_WEBHOOK_SECRET)
    except Exception as e:
        logger.error(f"Stripe webhook error: {e}")
        return JSONResponse(status_code=400, content={"error": str(e)})
    event_type = event.get("type")
    if event_type in ("customer.subscription.created", "customer.subscription.updated"):
        sub = event["data"]["object"]
        customer_id = sub["customer"]
        plan_id = sub["items"]["data"][0]["plan"]["id"]
        plan = STRIPE_PLAN_MAP.get(plan_id, "basic")
        redis_store.hset("customer_plans", customer_id, plan)
        logger.info(f"Set plan for {customer_id} to {plan}")
    elif event_type == "customer.subscription.deleted":
        sub = event["data"]["object"]
        customer_id = sub["customer"]
        redis_store.hset("customer_plans", customer_id, "basic")
        logger.info(f"Set plan for {customer_id} to basic (deleted)")
    else:
        logger.warning(f"Unhandled Stripe event: {event_type}")
    return {"status": "ok"}

@app.middleware("http")
async def enforce_plan_limits(request: Request, call_next):
    tenant = request.headers.get("x-tenant")
    if not tenant or not isinstance(tenant, str) or len(tenant) < 3:
        return JSONResponse(status_code=400, content={"error": "Missing or invalid x-tenant (Stripe customer ID) header."})
    customer_id = tenant
    plan = redis_store.hget("customer_plans", customer_id) or "basic"
    max_calls = PLANS.get(plan, PLANS["basic"])['max_calls']
    usage_key = f"usage:{customer_id}"
    usage = int(redis_store.get(usage_key) or 0)
    if request.url.path.startswith("/api/search") or request.url.path.startswith("/api/index"):
        if usage >= max_calls:
            return JSONResponse(status_code=402, content={"error": "Plan limit exceeded. Please upgrade your subscription."})
    response = await call_next(request)
    if request.url.path.startswith("/api/search") or request.url.path.startswith("/api/index"):
        redis_store.incr(usage_key)
        log_audit(request.url.path, customer_id, {"usage": usage+1, "plan": plan})
    return response

def log_audit(endpoint, user, params):
    entry = {
        "timestamp": datetime.utcnow().isoformat(),
        "endpoint": endpoint,
        "user": user,
        "params": params
    }
    with open("/app/audit.log", "a") as f:
        f.write(json.dumps(entry) + "\n")

def pseudonymize(text):
    # Masque les emails
    text = re.sub(r"[\w\.-]+@[\w\.-]+", "[EMAIL]", text)
    # Masque les numéros de téléphone (formats FR/EU simples)
    text = re.sub(r"\b\d{2} ?\d{2} ?\d{2} ?\d{2} ?\d{2}\b", "[PHONE]", text)
    # Masque les montants (euros, dollars)
    text = re.sub(r"\b\d+[.,]?\d* ?€|\$\d+[.,]?\d*\b", "[AMOUNT]", text)
    return text

class SummarizeRequest(BaseModel):
    text: str
    engine: str = "hf"  # "hf" ou "openai"
    openai_api_key: str = ""
    max_length: int = 130
    min_length: int = 30

class EntitiesRequest(BaseModel):
    text: str
    lang: str = "fr"
    engine: str = "spacy"  # ou "regex"

class FindSimilarRequest(BaseModel):
    doc_id: str
    top_k: int = 5
    mode: str = "simple"  # "simple" ou "grouped"
    threshold: float = 0.85

class AuditLLMRequest(BaseModel):
    engine: str = "local"  # "local", "openai", "hf"
    openai_api_key: str = ""
    min_years: int = 2

try:
    import redis
    import hashlib
    redis_client = redis.Redis(host="redis", port=6379, db=0, decode_responses=True)
except Exception:
    redis_client = None
    hashlib = None

@app.post("/api/index", response_model=dict)
@require_role("admin")
@limiter.limit("10/minute")
async def index_documents(request: Request, request_body: IndexRequest):
    tenant = request.headers.get("x-tenant", "default")
    docs_pseudo = [pseudonymize(doc) for doc in request_body.documents]
    log_audit("/api/index", "api", request_body.dict())
    t0 = time.time()
    try:
        doc_ids = search_engine.index_documents(
            documents=docs_pseudo,
            metadata=request_body.metadata,
            namespace=tenant
        )
        analytics["indexations"] += len(doc_ids)
        return {"success": True, "indexed": len(doc_ids), "document_ids": doc_ids}
    except Exception as e:
        raise HTTPException(500, str(e))

@app.post("/api/search")
@require_role("admin")
@limiter.limit("30/minute")
async def semantic_search(request: Request, request_body: SearchRequest):
    tenant = request.headers.get("x-tenant", "default")
    logger.debug(f"/api/search params: {request_body.dict()}")
    log_audit("/api/search", "api", request_body.dict())
    t0 = time.time()
    cache_key = None
    cached_result = None
    if redis_client and hashlib:
        try:
            key_raw = json.dumps(request_body.dict(), sort_keys=True) + tenant
            cache_key = "search:" + hashlib.sha256(key_raw.encode()).hexdigest()
            cached = redis_client.get(cache_key)
            if cached:
                logger.info(f"/api/search cache hit: {cache_key}")
                result = json.loads(cached)
                result["cached"] = True
                return JSONResponse(content=result)
            else:
                logger.info(f"/api/search cache miss: {cache_key}")
        except Exception as e:
            logger.warning(f"/api/search cache error: {str(e)}")
    try:
        results = search_engine.search(
            query=request_body.query,
            top_k=request_body.top_k,
            threshold=request_body.threshold,
            namespace=tenant,
            filters=request_body.filters
        )
        logger.info(f"/api/search returned {len(results['results'])} results")
        analytics["searches"] += 1
        analytics["top_queries"][request_body.query] += 1
        for r in results["results"]:
            analytics["top_docs"][r["id"]] += 1
        t1 = time.time()
        analytics["response_times"].append(t1-t0)
        analytics["last_search_time"] = t1-t0
        response = {
            "query": request_body.query,
            "results": results["results"],
            "processing_time_ms": results.get("time_ms", 0),
            "cached": False
        }
        if redis_client and cache_key:
            try:
                redis_client.setex(cache_key, 60, json.dumps(response))
            except Exception:
                pass
        return response
    except Exception as e:
        logger.error(f"/api/search error: {str(e)}", exc_info=True)
        raise HTTPException(500, str(e))

@app.get("/api/stats")
async def get_stats():
    stats = search_engine.get_stats(namespace="default")
    return stats

@app.get("/api/autocomplete", tags=["Autocomplete"])
async def autocomplete(prefix: str):
    base_suggestions = [
        "clause de non-concurrence",
        "clause abusive",
        "confidentialité",
        "jurisprudence cassation",
        "contrat SaaS",
        "email client"
    ]
    suggestions = [s for s in base_suggestions if s.startswith(prefix.lower())]
    if prefix.lower().startswith("clause"):
        suggestions += ["clause de confidentialité", "clause de résiliation"]
    return {"suggestions": suggestions[:10]}

@app.get("/api/duplicates")
async def get_duplicates(threshold: float = 0.95):
    log_audit("/api/duplicates", "api", {"threshold": threshold})
    groups = search_engine.find_duplicates(namespace="default", threshold=threshold)
    return groups

@app.post("/api/summarize")
@require_role("admin")
async def summarize(request: Request, req: SummarizeRequest = Body(...)):
    log_audit("/api/summarize", "api", req.dict())
    if req.engine == "hf":
        if summarizer is None:
            return {"error": "HuggingFace summarizer not available (transformers not installed?)"}
        summary = summarizer(req.text, max_length=req.max_length, min_length=req.min_length, do_sample=False)
        return {"summary": summary[0]["summary_text"]}
    elif req.engine == "openai":
        if not req.openai_api_key:
            return {"error": "OpenAI API key required"}
        headers = {"Authorization": f"Bearer {req.openai_api_key}", "Content-Type": "application/json"}
        data = {
            "model": "gpt-3.5-turbo",
            "messages": [
                {"role": "system", "content": "You are a legal document summarizer."},
                {"role": "user", "content": f"Résumé ce texte en 5 lignes maximum : {req.text}"}
            ],
            "max_tokens": 200
        }
        resp = safe_post("https://api.openai.com/v1/chat/completions", headers=headers, json=data)
        if resp.status_code == 200:
            summary = resp.json()["choices"][0]["message"]["content"]
            return {"summary": summary}
        else:
            return {"error": f"OpenAI error: {resp.text}"}
    else:
        return {"error": "Unknown engine"}

@app.post("/api/entities")
@require_role("admin")
async def extract_entities(request: Request, req: EntitiesRequest = Body(...)):
    log_audit("/api/entities", "api", req.dict())
    entities = {"dates": [], "money": [], "persons": [], "orgs": [], "locations": []}
    if req.engine == "spacy" and ((req.lang == "fr" and nlp_fr) or (req.lang == "en" and nlp_en)):
        nlp = nlp_fr if req.lang == "fr" else nlp_en
        doc = nlp(req.text)
        for ent in doc.ents:
            if ent.label_ in ["DATE"]:
                entities["dates"].append(ent.text)
            elif ent.label_ in ["MONEY"]:
                entities["money"].append(ent.text)
            elif ent.label_ in ["PER", "PERSON"]:
                entities["persons"].append(ent.text)
            elif ent.label_ in ["ORG"]:
                entities["orgs"].append(ent.text)
            elif ent.label_ in ["LOC", "GPE"]:
                entities["locations"].append(ent.text)
    else:
        # Fallback regex
        entities["dates"] = re.findall(r"\\d{4}-\\d{2}-\\d{2}|\\d{2}/\\d{2}/\\d{4}", req.text)
        entities["money"] = re.findall(r"\\d+[.,]?\\d* ?€|\\$\\d+[.,]?\\d*", req.text)
        entities["persons"] = re.findall(r"[A-Z][a-z]+ [A-Z][a-z]+", req.text)
    return entities

@app.post("/api/find_similar")
@require_role("admin")
async def find_similar(request: Request, req: FindSimilarRequest = Body(...)):
    log_audit("/api/find_similar", "api", req.dict())
    namespace = "default"
    # Cherche l'embedding du doc source
    collection = search_engine.chroma_client.get_collection(f"namespace_{namespace}")
    doc_data = collection.get(ids=[req.doc_id])
    if not doc_data["embeddings"]:
        return {"error": "Document not found"}
    emb = doc_data["embeddings"][0]
    import numpy as np
    emb = np.array(emb).reshape(1, -1).astype(np.float32)
    if namespace not in search_engine.indices:
        return {"results": []}
    index = search_engine.indices[namespace]
    faiss = __import__('faiss')
    faiss.normalize_L2(emb)
    D, I = index.search(emb, min(req.top_k+1, index.ntotal))
    doc_mapping = search_engine.doc_mappings[namespace]
    results = []
    for dist, idx in zip(D[0], I[0]):
        if idx == -1 or doc_mapping[idx] == req.doc_id:
            continue
        score = 1 - dist
        if score < req.threshold:
            continue
        doc_id = doc_mapping[idx]
        doc_data2 = collection.get(ids=[doc_id])
        results.append({
            "id": doc_id,
            "score": float(score),
            "text": doc_data2["documents"][0],
            "metadata": doc_data2["metadatas"][0]
        })
        if len(results) >= req.top_k:
            break
    if req.mode == "simple":
        return {"results": results}
    # Mode grouped : regroupe les docs très proches
    groups = []
    used = set()
    for r in results:
        if r["id"] in used:
            continue
        group = [r]
        for r2 in results:
            if r2["id"] == r["id"] or r2["id"] in used:
                continue
            if abs(r["score"] - r2["score"]) < 0.05:
                group.append(r2)
                used.add(r2["id"])
        used.add(r["id"])
        if len(group) > 1:
            groups.append(group)
    return {"results": results, "groups": groups}

@app.get("/api/analytics")
@require_role("admin")
async def get_analytics(request: Request):
    avg_time = sum(analytics["response_times"])/len(analytics["response_times"]) if analytics["response_times"] else 0
    top_queries = sorted(analytics["top_queries"].items(), key=lambda x: -x[1])[:10]
    top_docs = sorted(analytics["top_docs"].items(), key=lambda x: -x[1])[:10]
    return {
        "searches": analytics["searches"],
        "indexations": analytics["indexations"],
        "avg_response_time_ms": round(avg_time*1000, 2),
        "last_search_time_ms": round(analytics["last_search_time"]*1000, 2),
        "top_queries": top_queries,
        "top_docs": top_docs
    }

@app.get("/api/audit")
async def audit_non_concurrence(min_years: int = 2):
    namespace = "default"
    collection = search_engine.chroma_client.get_collection(f"namespace_{namespace}")
    # Récupère tous les documents
    all_docs = collection.get()["documents"]
    all_ids = collection.get()["ids"]
    all_metas = collection.get()["metadatas"]
    non_conformes = []
    for i, doc in enumerate(all_docs):
        # Heuristique simple : cherche "non-concurrence" et durée > min_years
        if "non-concurrence" in doc.lower():
            import re
            match = re.search(r"(\d+) ?ans", doc)
            if match and int(match.group(1)) > min_years:
                non_conformes.append({
                    "id": all_ids[i],
                    "text": doc,
                    "metadata": all_metas[i],
                    "duree": int(match.group(1))
                })
    rapport = {
        "total_docs": len(all_docs),
        "non_concurrence_trouvees": len(non_conformes),
        "docs_non_conformes": non_conformes,
        "recommandation": f"{len(non_conformes)} clause(s) de non-concurrence dépassent {min_years} ans. Vérifiez leur conformité."
    }
    return rapport

@app.post("/api/audit_llm")
async def audit_llm(req: AuditLLMRequest = Body(...)):
    namespace = "default"
    collection = search_engine.chroma_client.get_collection(f"namespace_{namespace}")
    all_docs = collection.get()["documents"]
    all_ids = collection.get()["ids"]
    all_metas = collection.get()["metadatas"]
    non_conformes = []
    if req.engine == "local":
        # Même logique que /api/audit
        for i, doc in enumerate(all_docs):
            if "non-concurrence" in doc.lower():
                import re
                match = re.search(r"(\d+) ?ans", doc)
                if match and int(match.group(1)) > req.min_years:
                    non_conformes.append({
                        "id": all_ids[i],
                        "text": doc,
                        "metadata": all_metas[i],
                        "duree": int(match.group(1))
                    })
        rapport = {
            "total_docs": len(all_docs),
            "non_concurrence_trouvees": len(non_conformes),
            "docs_non_conformes": non_conformes,
            "recommandation": f"{len(non_conformes)} clause(s) de non-concurrence dépassent {req.min_years} ans. Vérifiez leur conformité."
        }
        return rapport
    elif req.engine == "openai":
        if not req.openai_api_key:
            return {"error": "OpenAI API key required"}
        import requests as pyrequests
        headers = {"Authorization": f"Bearer {req.openai_api_key}", "Content-Type": "application/json"}
        flagged = []
        for i, doc in enumerate(all_docs):
            data = {
                "model": "gpt-3.5-turbo",
                "messages": [
                    {"role": "system", "content": "You are a legal compliance auditor. Detect if this clause is non-compliant (non-concurrence > 2 years) and explain why."},
                    {"role": "user", "content": doc}
                ],
                "max_tokens": 100
            }
            resp = safe_post("https://api.openai.com/v1/chat/completions", headers=headers, json=data)
            if resp.status_code == 200:
                answer = resp.json()["choices"][0]["message"]["content"].lower()
                if "non-compliant" in answer or "plus de 2 ans" in answer or "not compliant" in answer:
                    flagged.append({"id": all_ids[i], "text": doc, "metadata": all_metas[i], "llm": answer})
        return {"total_docs": len(all_docs), "flagged": len(flagged), "docs_flagged": flagged}
    else:
        return {"error": "Engine not supported"}

class VeilleRequest(BaseModel):
    engine: str = "local"  # "local", "openai", "hf"
    openai_api_key: str = ""
    keyword: str = "jurisprudence"

@app.post("/api/veille")
async def veille_agent(req: VeilleRequest = Body(...)):
    namespace = "default"
    collection = search_engine.chroma_client.get_collection(f"namespace_{namespace}")
    all_docs = collection.get()["documents"]
    all_ids = collection.get()["ids"]
    all_metas = collection.get()["metadatas"]
    alerts = []
    if req.engine == "local":
        for i, doc in enumerate(all_docs):
            if req.keyword.lower() in doc.lower():
                alerts.append({"id": all_ids[i], "text": doc, "metadata": all_metas[i]})
        return {"total_docs": len(all_docs), "alerts": len(alerts), "docs_alerts": alerts}
    elif req.engine == "openai":
        if not req.openai_api_key:
            return {"error": "OpenAI API key required"}
        import requests as pyrequests
        headers = {"Authorization": f"Bearer {req.openai_api_key}", "Content-Type": "application/json"}
        flagged = []
        for i, doc in enumerate(all_docs):
            data = {
                "model": "gpt-3.5-turbo",
                "messages": [
                    {"role": "system", "content": f"You are a legal watch agent. Detect if this document contains new jurisprudence or legal risk related to {req.keyword}."},
                    {"role": "user", "content": doc}
                ],
                "max_tokens": 100
            }
            resp = safe_post("https://api.openai.com/v1/chat/completions", headers=headers, json=data)
            if resp.status_code == 200:
                answer = resp.json()["choices"][0]["message"]["content"].lower()
                if "alert" in answer or "nouvelle jurisprudence" in answer or "risk" in answer:
                    flagged.append({"id": all_ids[i], "text": doc, "metadata": all_metas[i], "llm": answer})
        return {"total_docs": len(all_docs), "alerts": len(flagged), "docs_alerts": flagged}
    else:
        return {"error": "Engine not supported"}

@app.get("/api/ready")
async def ready():
    """Readiness check endpoint for readiness probe."""
    # Optionally check DB, model, etc.
    return {"ready": True}

@app.get("/api/audit_log")
@require_role("admin")
async def export_audit_log(request: Request):
    try:
        with open("/app/audit.log", "r") as f:
            lines = f.readlines()
        if not lines:
            return StreamingResponse(io.StringIO("timestamp,endpoint,user,params\n"), media_type="text/csv")
        import csv
        output = io.StringIO()
        writer = csv.writer(output)
        writer.writerow(["timestamp", "endpoint", "user", "params"])
        for line in lines:
            entry = json.loads(line)
            writer.writerow([
                entry.get("timestamp", ""),
                entry.get("endpoint", ""),
                entry.get("user", ""),
                json.dumps(entry.get("params", {}))
            ])
        output.seek(0)
        return StreamingResponse(output, media_type="text/csv")
    except Exception as e:
        return JSONResponse(status_code=500, content={"error": str(e)})

@app.get("/api/logs")
@require_role("admin")
async def get_logs(request: Request):
    try:
        import subprocess
        logs = subprocess.check_output(["tail", "-n", "100", "/proc/1/fd/1"]).decode()
        return {"logs": logs}
    except Exception as e:
        return {"error": str(e)}

@app.exception_handler(429)
async def rate_limit_exceeded(request, exc):
    return JSONResponse(status_code=429, content={"error": "Rate limit exceeded"})

def safe_post(*args, **kwargs):
    kwargs.setdefault("timeout", 10)
    return pyrequests.post(*args, **kwargs)

# --- Deployment & Monitoring ---
# - Use /api/health and /api/ready for liveness/readiness probes
# - Prometheus metrics at /metrics (scraped by prometheus.yml)
# - Grafana dashboards recommended for advanced monitoring
# --- End Deployment & Monitoring ---

# --- Testing ---
# See tests/ for unit and integration tests (pytest)
# Run: pytest tests/
# --- End Testing ---

"""
Semantic Search API
Production-ready API for legal/compliance semantic search with security, validation, monitoring, and audit.
"""

explainer = SearchExplainer(search_engine.model, BasicTokenizer())

class ExplanationRequest(BaseModel):
    query: str
    document: str

@app.post("/api/explain")
async def explain_search(req: ExplanationRequest):
    """Retourne une explication SHAP pour un document donné et une requête."""
    return explainer.explain(req.query, req.document) 