import pytest
from httpx import AsyncClient
import asyncio
from fastapi.testclient import TestClient
from app import app

API_URL = "http://localhost:8000"

client = TestClient(app)

API_KEY = "dev-key"
HEADERS = {"x-api-key": API_KEY, "x-role": "admin"}

@pytest.mark.asyncio
async def test_health():
    async with AsyncClient(base_url=API_URL) as ac:
        resp = await ac.get("/api/health")
        assert resp.status_code == 200
        assert resp.json()["status"] == "ok"

@pytest.mark.asyncio
async def test_index_auth():
    async with AsyncClient(base_url=API_URL) as ac:
        # Sans API key
        resp = await ac.post("/api/index", json={"documents": ["test"], "metadata": [{"type": "test"}]})
        assert resp.status_code == 401
        # Avec API key mais sans rôle admin
        resp = await ac.post("/api/index", json={"documents": ["test"], "metadata": [{"type": "test"}]}, headers={"x-api-key": "test-key", "x-role": "user"})
        assert resp.status_code == 403
        # Avec API key et rôle admin
        resp = await ac.post("/api/index", json={"documents": ["test"], "metadata": [{"type": "test"}]}, headers={"x-api-key": "test-key", "x-role": "admin"})
        assert resp.status_code == 200
        assert resp.json()["success"] is True

def test_index_validation():
    # Too long document
    data = {"documents": ["a"*10001], "metadata": [{}]}
    r = client.post("/api/index", json=data, headers=HEADERS)
    assert r.status_code == 422
    # Too many documents
    data = {"documents": ["a"]*101, "metadata": [{}]*101}
    r = client.post("/api/index", json=data, headers=HEADERS)
    assert r.status_code == 422 