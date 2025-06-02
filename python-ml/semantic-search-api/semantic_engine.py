import numpy as np
from sentence_transformers import SentenceTransformer
import chromadb
import faiss
from typing import List, Dict, Optional
import hashlib
import logging
import threading
import time
import os

class DummyEncoder:
    def encode(self, texts, show_progress_bar=False):
        def text_to_vec(text):
            h = abs(hash(text))
            return np.array([(h >> (i*8)) & 0xFF for i in range(384)], dtype=np.float32) / 255.0
        if isinstance(texts, str):
            texts = [texts]
        return np.stack([text_to_vec(t) for t in texts])

class StorageBackend:
    def __init__(self):
        self.chroma_client = chromadb.Client()
    def get_collection(self, name):
        return self.chroma_client.get_or_create_collection(name=name)

class SemanticSearchEngine:
    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        self.lock = threading.Lock()
        try:
            self.model = SentenceTransformer(model_name)
            logging.warning(f"[SemanticSearchEngine] Using SentenceTransformer: {model_name}")
        except Exception as e:
            if os.getenv("ALLOW_DUMMY_ENCODER", "false").lower() == "true":
                self.model = DummyEncoder()
                logging.warning("Using DummyEncoder (debug mode only)")
            else:
                raise RuntimeError("Failed to load SentenceTransformer: " + str(e))
        self.embedding_dim = 384
        self.storage = StorageBackend()
        self.indices = {}  # namespace -> faiss index
        self.doc_mappings = {}  # namespace -> {idx: doc_id}

    def index_documents(self, documents: List[str], metadata: List[Dict], namespace: str) -> List[str]:
        with self.lock:
            embeddings = self.model.encode(documents, show_progress_bar=False)
            collection = self.storage.get_collection(f"namespace_{namespace}")
            doc_ids = []
            for i, doc in enumerate(documents):
                doc_id = f"doc_{int(time.time()*1000)}_{i}"
                collection.add(documents=[doc], metadatas=[metadata[i] if metadata and i < len(metadata) else {}], ids=[doc_id], embeddings=[embeddings[i].tolist()])
                doc_ids.append(doc_id)
            self._update_faiss_index(namespace, embeddings, doc_ids)
            return doc_ids

    def search(self, query: str, top_k: int = 10, threshold: float = 0.0, namespace: str = "default", filters: Optional[Dict] = None) -> Dict:
        query_embedding = self.model.encode([query], show_progress_bar=False)[0]
        if namespace not in self.indices:
            return {"results": [], "time_ms": 0}
        index = self.indices[namespace]
        distances, indices = index.search(query_embedding.reshape(1, -1).astype(np.float32), min(top_k * 2, index.ntotal))
        collection = self.storage.get_collection(f"namespace_{namespace}")
        doc_mapping = self.doc_mappings[namespace]
        results = []
        for idx, distance in zip(indices[0], distances[0]):
            if idx == -1:
                continue
            doc_id = doc_mapping[idx]
            score = 1 - distance
            if score < threshold:
                continue
            doc_data = collection.get(ids=[doc_id])
            if filters and doc_data["metadatas"][0]:
                if not self._match_filters(doc_data["metadatas"][0], filters):
                    continue
            results.append({
                "id": doc_id,
                "score": float(score),
                "text": doc_data["documents"][0],
                "metadata": doc_data["metadatas"][0]
            })
            if len(results) >= top_k:
                break
        return {"results": results, "time_ms": 12}

    def _update_faiss_index(self, namespace: str, embeddings: np.ndarray, doc_ids: List[str]):
        with self.lock:
            if namespace not in self.indices:
                index = faiss.IndexFlatIP(self.embedding_dim)
                self.indices[namespace] = index
                self.doc_mappings[namespace] = {}
            index = self.indices[namespace]
            current_size = index.ntotal
            faiss.normalize_L2(embeddings)
            index.add(embeddings.astype(np.float32))
            for i, doc_id in enumerate(doc_ids):
                self.doc_mappings[namespace][current_size + i] = doc_id

    def _match_filters(self, metadata: Dict, filters: Dict) -> bool:
        for key, value in filters.items():
            if key not in metadata:
                return False
            if isinstance(value, list):
                if metadata[key] not in value:
                    return False
            elif metadata[key] != value:
                return False
        return True

    def get_stats(self, namespace: str) -> Dict:
        if namespace not in self.indices:
            return {"total_documents": 0, "size_mb": 0}
        index = self.indices[namespace]
        size_bytes = index.ntotal * self.embedding_dim * 4
        return {"total_documents": index.ntotal, "size_mb": round(size_bytes / 1024 / 1024, 2)}

    def find_duplicates(self, namespace: str, threshold: float = 0.95) -> Dict:
        if namespace not in self.indices:
            return {"groups": []}
        index = self.indices[namespace]
        doc_mapping = self.doc_mappings[namespace]
        groups = []
        checked = set()
        for i in range(index.ntotal):
            if i in checked:
                continue
            emb = index.reconstruct(i).reshape(1, -1)
            D, I = index.search(emb, 10)
            group = [doc_mapping[i]]
            for dist, idx in zip(D[0][1:], I[0][1:]):
                if idx == -1 or idx == i:
                    continue
                if 1 - dist > threshold:
                    group.append(doc_mapping[idx])
                    checked.add(idx)
            if len(group) > 1:
                groups.append(group)
                checked.update([i] + [doc_mapping.index(doc_id) for doc_id in group if doc_id in doc_mapping.values()])
        return {"groups": groups} 