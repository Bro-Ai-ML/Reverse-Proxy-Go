import shap
import numpy as np
from sentence_transformers import SentenceTransformer
from typing import Dict

class BasicTokenizer:
    def tokenize(self, text):
        return text.split()

class SearchExplainer:
    def __init__(self, model, tokenizer):
        self.model = model
        self.tokenizer = tokenizer
        self.explainer = shap.Explainer(self.model.encode)
    
    def explain(self, query: str, document: str) -> Dict:
        # Génère les explications sur l'embedding du document
        shap_values = self.explainer([document])
        tokens = self.tokenizer.tokenize(document)
        values = shap_values.values[0]
        return {
            "tokens": tokens,
            "importance_scores": values.tolist(),
            "base_value": float(np.mean(shap_values.base_values))
        } 