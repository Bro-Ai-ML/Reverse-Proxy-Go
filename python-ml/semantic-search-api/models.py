from pydantic import BaseModel, Field, validator
from typing import List, Dict, Optional

class IndexRequest(BaseModel):
    documents: List[str] = Field(..., max_items=100)
    metadata: Optional[List[Dict]] = None
    @validator("documents", each_item=True)
    def check_doc_length(cls, v):
        if len(v) > 10000:
            raise ValueError("Document too long (max 10,000 chars)")
        return v

class SearchRequest(BaseModel):
    query: str
    top_k: int = Field(5, ge=1, le=20)
    threshold: float = 0.0
    filters: Optional[Dict] = None
    @validator("query")
    def check_query_length(cls, v):
        if len(v) > 500:
            raise ValueError("Query too long (max 500 chars)")
        return v

class SearchResult(BaseModel):
    id: str
    score: float
    text: str
    metadata: Dict

class SearchResponse(BaseModel):
    query: Optional[str] = None
    results: List[SearchResult]
    processing_time_ms: int

"""
Models for Semantic Search API
""" 