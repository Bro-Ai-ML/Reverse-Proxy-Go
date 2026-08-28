#!/usr/bin/env python
"""Dynamique temporelle (questions 7 & 8).

Q7 : quand le stockage dépasse-t-il la capacité de ramping du gaz restant ?
Q8 : où la pointe du soir reste-t-elle 100 % fossile malgré les GW solaires ?
"""

from __future__ import annotations

import polars as pl

from energyeu.config import ACTIVE_STATUSES, PIPELINE_STATUSES
from energyeu.load import OUT_DIR, ensure_processed, europe_grid
from energyeu.peak import PEAK_DEMAND_GW

TABLES = OUT_DIR / "tables"
FIGURES = OUT_DIR / "figures"
TABLES.mkdir(parents=True, exist_ok=True)
FIGURES.mkdir(parents=True, exist_ok=True)

# Facteurs d'utilisation typiques un soir d'hiver à 19h (fractions de capacité)
EVENING_AVAIL = {
    "utility-scale solar": 0.02,   # quasi zéro en hiver
    "wind": 0.25,
    "nuclear": 0.90,
    "hydropower": 0.45,
    "bioenergy": 0.60,
    "geothermal": 0.85,
}


def main() -> None:
    df = ensure_processed()
    eur = europe_grid(df)
    op = eur.filter(pl.col("status").is_in(ACTIVE_STATUSES))

    # ------------------------------------------------------------------
    # Q8 — pointe du soir encore fossile
    # ------------------------------------------------------------------
    by_country = (
        op.group_by("iso3", "type")
        .agg(pl.col("capacity_mw").sum().alias("mw"))
        .pivot(on="type", index="iso3", values="mw", aggregate_function="sum")
        .fill_null(0.0)
    )

    rows = []
    for r in by_country.iter_rows(named=True):
        iso = r["iso3"]
        peak = PEAK_DEMAND_GW.get(iso)
        if peak is None:
            continue
        solar = r.get("utility-scale solar", 0.0) / 1e3
        wind = r.get("wind", 0.0) / 1e3
        fossil = (r.get("oil/gas", 0.0) + r.get("coal", 0.0)) / 1e3
        clean_evening = sum(r.get(t, 0.0) / 1e3 * f for t, f in EVENING_AVAIL.items())
        # stockage (pompage) disponible le soir
        storage = float(
            op.filter(pl.col("iso3") == iso)
            .filter(pl.col("Technology").str.contains("pumped", literal=False))
            ["capacity_mw"].sum()
        ) / 1e3
        clean_evening += storage * 0.5
        evening_need = max(0.0, 0.9 * peak - clean_evening)  # type: ignore[operator]
        rows.append({
            "iso3": iso,
            "peak_gw": peak,
            "solar_gw": solar,
            "wind_gw": wind,
            "fossil_gw": fossil,
            "storage_gw": storage,
            "solar_peak_share": solar / peak,
            "evening_fossil_need_gw": evening_need,
            "evening_fossil_index": evening_need / (0.9 * peak),
            "fossil_evening_gap_gw": max(0.0, evening_need - fossil),
        })
    q8 = pl.DataFrame(rows).sort("evening_fossil_index", descending=True)
    q8.write_csv(TABLES / "q8_evening_fossil.csv")

    # ------------------------------------------------------------------
    # Q7 — stockage vs gaz : croisement déjà atteint ?
    # ------------------------------------------------------------------
    gas = (
        eur.filter(pl.col("type") == "oil/gas")
        .group_by("iso3")
        .agg(
            pl.col("capacity_mw").filter(pl.col("status").is_in(ACTIVE_STATUSES))
            .sum().alias("gas_op_mw"),
            pl.col("capacity_mw").filter(pl.col("status").is_in(PIPELINE_STATUSES))
            .sum().alias("gas_pipe_mw"),
        )
    )
    # Stockage GEM : pompage + solaire avec stockage associé
    eur = eur.with_columns(
        pl.when(
            pl.col("Technology").str.contains("pumped", literal=False)
            | (pl.col("Associated storage") == "yes")
        ).then(pl.lit(True)).otherwise(pl.lit(False)).alias("is_storage")
    )
    storage_all = (
        eur.filter(pl.col("is_storage"))
        .group_by("iso3")
        .agg(
            pl.col("capacity_mw").filter(pl.col("status").is_in(ACTIVE_STATUSES))
            .sum().alias("storage_op_mw"),
            pl.col("capacity_mw").filter(pl.col("status").is_in(PIPELINE_STATUSES))
            .sum().alias("storage_pipe_mw"),
        )
    )
    q7 = (
        gas.join(storage_all, on="iso3", how="left")
        .fill_null(0.0)
        .with_columns(
            (pl.col("storage_op_mw") + pl.col("storage_pipe_mw")).alias("storage_total_mw"),
            (pl.col("gas_op_mw") - (pl.col("storage_op_mw") + pl.col("storage_pipe_mw")))
            .alias("gas_excess_mw"),
        )
        .sort("gas_op_mw", descending=True)
    )
    q7.write_csv(TABLES / "q7_storage_vs_gas.csv")

    # Série temporelle cumulée de stockage (pour le modèle bayésien)
    # Restreint aux unités réellement déployées (operating) ou en construction.
    built = ACTIVE_STATUSES | {"construction"}
    stor_hist = (
        eur.filter(pl.col("is_storage"))
        .filter(pl.col("status").is_in(built))
        .filter(pl.col("year").is_not_null() & (pl.col("year") >= 1990))
        .with_columns(pl.col("year").clip(1990, 2026))
        .group_by("year")
        .agg(pl.col("capacity_mw").sum().alias("mw_commissioned"))
        .sort("year")
        .with_columns(pl.col("mw_commissioned").cum_sum().alias("mw_cum"))
    )
    stor_hist.write_csv(TABLES / "q7_storage_vintage.csv")

    gas_hist = (
        eur.filter(pl.col("type") == "oil/gas")
        .filter(pl.col("status").is_in(built))
        .filter(pl.col("year").is_not_null() & (pl.col("year") >= 1990))
        .with_columns(pl.col("year").clip(1990, 2026))
        .group_by("year")
        .agg(pl.col("capacity_mw").sum().alias("mw_commissioned"))
        .sort("year")
        .with_columns(pl.col("mw_commissioned").cum_sum().alias("mw_cum"))
    )
    gas_hist.write_csv(TABLES / "q7_gas_vintage.csv")

    # ------------------------------------------------------------------
    # Sorties
    # ------------------------------------------------------------------
    print("=== Q8 — soir encore fossile (index, top 15) ===")
    print(
        q8.select(["iso3", "peak_gw", "solar_gw", "fossil_gw", "storage_gw",
                   "evening_fossil_index"])
        .head(15).to_pandas().to_string(index=False)
    )
    print("\n=== Q7 — stockage (pompage+co-loc) vs gaz (GW, top 15) ===")
    print(
        q7.select(["iso3", "gas_op_mw", "gas_pipe_mw", "storage_op_mw",
                   "storage_pipe_mw", "gas_excess_mw"])
        .head(15).to_pandas().to_string(index=False)
    )
    print("\nStockage cumulé Europe (GEM-tracked) 2020-2026 (GW):")
    print(
        stor_hist.filter(pl.col("year") >= 2020)
        .with_columns((pl.col("mw_cum") / 1e3).round(2))
        .select(["year", "mw_cum"])
        .to_pandas().to_string(index=False)
    )


if __name__ == "__main__":
    main()
