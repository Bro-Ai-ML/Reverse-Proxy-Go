"""Chargement et nettoyage des données GEM / GTD."""

from __future__ import annotations

from pathlib import Path

import polars as pl

from energyeu.config import (
    ACTIVE_STATUSES,
    GEM_NAME_TO_ISO3,
    GREATER_EUROPE,
    NORTH_AFRICA,
    SILK_ROAD,
)


def _num(v: object) -> float | None:
    """Convertit une valeur GTD ('-' = manquant) en float ou None."""
    if v is None or v == "-" or v == "":
        return None
    try:
        return float(v)  # type: ignore[arg-type]
    except (TypeError, ValueError):
        return None


RAW_DIR = Path(__file__).resolve().parents[2] / "data" / "raw"
PROC_DIR = Path(__file__).resolve().parents[2] / "data" / "processed"
OUT_DIR = Path(__file__).resolve().parents[2] / "output"

GIPT_PARQUET = RAW_DIR / "gipt_aug2026.parquet"


def load_gipt() -> pl.DataFrame:
    """Charge le GIPT (Global Integrated Power Tracker) depuis le parquet."""
    df = pl.read_parquet(GIPT_PARQUET)
    return df.with_columns(
        pl.col("Capacity (MW)").cast(pl.Float64).alias("capacity_mw"),
        pl.col("Start year").cast(pl.String).alias("start_year_raw"),
        pl.col("Latitude").cast(pl.Float64),
        pl.col("Longitude").cast(pl.Float64),
    )


def add_iso3(df: pl.DataFrame) -> pl.DataFrame:
    """Ajoute la colonne iso3 à partir des noms de pays GEM."""
    mapping = pl.DataFrame(
        {"Country/area": list(GEM_NAME_TO_ISO3), "iso3": list(GEM_NAME_TO_ISO3.values())}
    )
    return df.join(mapping, on="Country/area", how="left")


def clean(df: pl.DataFrame) -> pl.DataFrame:
    """Nettoyage : iso3, années, statuts normalisés."""
    df = add_iso3(df)

    def _to_year(s: pl.Expr) -> pl.Expr:
        return (
            s.str.extract(r"(\d{4})", 1)
            .cast(pl.Int64)
            .alias("year")
        )

    return df.with_columns(
        _to_year(pl.col("start_year_raw")),
        pl.col("Retired year").cast(pl.Int64).alias("retired_year"),
        pl.col("Status").str.strip_chars().alias("status"),
        pl.col("Type").str.strip_chars().alias("type"),
    )


def europe_grid(df: pl.DataFrame) -> pl.DataFrame:
    """Filtre sur le périmètre 'grille européenne' (EU + voisins + Türkiye/Caucase)."""
    return df.filter(pl.col("iso3").is_in(GREATER_EUROPE))


def north_africa(df: pl.DataFrame) -> pl.DataFrame:
    return df.filter(pl.col("iso3").is_in(NORTH_AFRICA))


def silk_road(df: pl.DataFrame) -> pl.DataFrame:
    return df.filter(pl.col("iso3").is_in(SILK_ROAD))


def capacity_by(df: pl.DataFrame, by: list[str]) -> pl.DataFrame:
    """Capacité (MW) et nombre d'unités par dimensions données."""
    return (
        df.group_by(by)
        .agg(
            pl.len().alias("n_units"),
            pl.col("capacity_mw").sum().alias("capacity_mw"),
        )
        .sort("capacity_mw", descending=True)
    )


def load_gtd() -> pl.DataFrame:
    """Charge la base d'interconnexions transnationales (existant + planifié)."""
    import openpyxl  # noqa: PLC0415

    wb = openpyxl.load_workbook(
        RAW_DIR / "global_transmission_data.xlsx", read_only=True, data_only=True
    )

    rows: list[dict[str, object]] = []
    for sheet, kind in (("GTD-v1.0_regional_existing", "existing"),
                        ("GTD-v1.0_regional_planned", "planned")):
        ws = wb[sheet]
        it = ws.iter_rows(values_only=True)
        header = next(it)
        idx = {name: i for i, name in enumerate(header)}
        for r in it:
            if r[idx["from_country"]] is None:
                continue
            rows.append({
                "from_iso3": r[idx["from_country"]],
                "to_iso3": r[idx["to_country"]],
                "kind": kind,
                "max_flow_mw": _num(r[idx["max_flow"]]),
                "min_flow_mw": _num(r[idx["min_flow"]]),
                "voltage_kv": _num(r[idx["voltage"]]),
                "distance_km": _num(r[idx["distance"]]),
                "year_planned": _num(r[idx["year_planned"]]) if kind == "planned" else None,
            })
    return pl.DataFrame(rows, schema={
        "from_iso3": pl.String,
        "to_iso3": pl.String,
        "kind": pl.String,
        "max_flow_mw": pl.Float64,
        "min_flow_mw": pl.Float64,
        "voltage_kv": pl.Float64,
        "distance_km": pl.Float64,
        "year_planned": pl.Float64,
    })


def load_solar_tracker() -> pl.DataFrame:
    """Charge le Global Solar Power Tracker (si disponible)."""
    path = RAW_DIR / "Global-Solar-Power-Tracker-February-2026.xlsx"
    if not path.exists():
        return pl.DataFrame()
    return pl.read_excel(path, engine="openpyxl")


def ensure_processed(force: bool = False) -> pl.DataFrame:
    """Construit (et cache) le dataframe GIPT nettoyé + filtre Europe."""
    proc_path = PROC_DIR / "gipt_clean.parquet"
    if proc_path.exists() and not force:
        return pl.read_parquet(proc_path)
    PROC_DIR.mkdir(parents=True, exist_ok=True)
    df = clean(load_gipt())
    df.write_parquet(proc_path)
    return df


def status_class(status: str) -> str:
    """Classe agrégée d'un statut GEM."""
    if status in ACTIVE_STATUSES:
        return "operating"
    if status in {"construction", "pre-construction", "announced"}:
        return "pipeline"
    if status in {"retired", "cancelled", "cancelled - inferred 4 y",
                  "shelved - inferred 2 y", "mothballed"}:
        return "dead"
    if status in {"shelved"}:
        return "shelved"
    return "other"


STATUS_CLASSES = {
    "operating": "operating",
    "construction": "pipeline",
    "pre-construction": "pipeline",
    "announced": "pipeline",
    "retired": "dead",
    "cancelled": "dead",
    "cancelled - inferred 4 y": "dead",
    "shelved - inferred 2 y": "dead",
    "mothballed": "dead",
    "shelved": "shelved",
}
