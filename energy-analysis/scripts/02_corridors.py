#!/usr/bin/env python
"""Corridors d'interconnexion transnationale (thème central + question 5).

S'appuie sur la Global Transmission Database (GTD v1.0, PyPSA-Earth/OET) :
interconnexions existantes (2023) et planifiées par paire de pays.
"""

from __future__ import annotations

import polars as pl

from energyeu.config import CENTRAL_ASIA, GREATER_EUROPE, MIDDLE_EAST, NORTH_AFRICA
from energyeu.load import OUT_DIR, ensure_processed, europe_grid, load_gtd

TABLES = OUT_DIR / "tables"
TABLES.mkdir(parents=True, exist_ok=True)

# Pays hors "grille Europe" mais pertinents pour les corridors européens
RING = NORTH_AFRICA | CENTRAL_ASIA | MIDDLE_EAST | {"CHN", "PAK", "AFG", "ISR", "LBY", "SDN", "ETH", "NGA", "MRT"}


def main() -> None:
    gtd = load_gtd()
    df = ensure_processed()
    eur = europe_grid(df)

    # Capacité renouvelable (opérante + pipeline) par pays
    ren = (
        eur.filter(pl.col("type").is_in(["wind", "utility-scale solar", "hydropower"]))
        .group_by("iso3")
        .agg(
            pl.col("capacity_mw")
            .filter(pl.col("status") == "operating")
            .sum()
            .alias("ren_op_mw"),
            pl.col("capacity_mw")
            .filter(pl.col("status").is_in(["construction", "pre-construction", "announced"]))
            .sum()
            .alias("ren_pipeline_mw"),
        )
    )

    # ------------------------------------------------------------------
    # Corridors impliquant la grille européenne (existant + planifié)
    # ------------------------------------------------------------------
    eu_lines = gtd.filter(
        pl.col("from_iso3").is_in(GREATER_EUROPE) | pl.col("to_iso3").is_in(GREATER_EUROPE)
    )

    pivot = (
        eu_lines.pivot(
            on="kind",
            index=["from_iso3", "to_iso3"],
            values="max_flow_mw",
            aggregate_function="sum",
        )
        .fill_null(0.0)
        .rename({"existing": "existing_mw", "planned": "planned_mw"})
        .with_columns(
            (pl.col("planned_mw") / (pl.col("existing_mw") + 1.0)).alias("multiplier"),
            (pl.col("planned_mw") - pl.col("existing_mw")).alias("delta_mw"),
        )
        .sort("planned_mw", descending=True)
    )
    pivot.write_csv(TABLES / "corridors_europe_gtp.csv")

    # Nouveaux corridors (pas d'existant, planifié > 0) — « routes de la soie »
    new_corridors = pivot.filter((pl.col("existing_mw") == 0.0) & (pl.col("planned_mw") > 0.0))
    new_corridors.write_csv(TABLES / "corridors_new_only.csv")

    # Corridors transrégionaux majeurs (agrégation par région)
    region_map = {}
    for iso in GREATER_EUROPE:
        region_map[iso] = "Europe"
    for iso in NORTH_AFRICA:
        region_map[iso] = "NorthAfrica"
    for iso in CENTRAL_ASIA:
        region_map[iso] = "CentralAsia"
    for iso in MIDDLE_EAST:
        region_map[iso] = "MiddleEast"
    region_map["CHN"] = "China"

    gtd_reg = gtd.with_columns(
        pl.col("from_iso3").replace_strict(region_map, default="Other").alias("from_reg"),
        pl.col("to_iso3").replace_strict(region_map, default="Other").alias("to_reg"),
    ).filter(pl.col("from_reg") != "Other")

    reg_lines = (
        gtd_reg.group_by("from_reg", "to_reg", "kind")
        .agg(pl.col("max_flow_mw").sum().alias("mw"))
        .pivot(on="kind", index=["from_reg", "to_reg"], values="mw", aggregate_function="sum")
        .fill_null(0.0)
        .rename({"existing": "existing_mw", "planned": "planned_mw"})
        .with_columns((pl.col("planned_mw") - pl.col("existing_mw")).alias("delta_mw"))
        .sort("delta_mw", descending=True)
    )
    reg_lines.write_csv(TABLES / "corridors_regional.csv")

    # ------------------------------------------------------------------
    # Q5 — corridors où une ligne démultiplierait la valeur des centrales
    # ------------------------------------------------------------------
    # Proxies :
    #  - congestion : renouvelable opérante côté source / capacité de ligne existante
    #  - levier : capacité planifiée / existante
    #  - pipeline renouvelable en attente côté source
    both = (
        pivot.join(ren, left_on="from_iso3", right_on="iso3", how="left")
        .rename({"ren_op_mw": "ren_op_src_mw", "ren_pipeline_mw": "ren_pipe_src_mw"})
        .fill_null(0.0)
        .with_columns(
            # risque de congestion/curtailment : renouvelable source / capacité de ligne existante
            (pl.col("ren_op_src_mw") / (pl.col("existing_mw") + 1.0)).alias("curtail_risk"),
            # potentiel de déblocage : renouvelable (op + pipeline) par MW de ligne planifiée
            (
                (pl.col("ren_op_src_mw") + pl.col("ren_pipe_src_mw"))
                / (pl.col("planned_mw") + 1.0)
            ).alias("unlock_potential"),
            (pl.col("ren_pipe_src_mw") / (pl.col("planned_mw") + 1.0)).alias("pipe_per_line_mw"),
        )
        .filter((pl.col("planned_mw") > 0.0) & (pl.col("ren_op_src_mw") > 0.0))
        .with_columns(
            (
                pl.col("curtail_risk").log1p() * pl.col("planned_mw").log1p() / 1e3
            ).alias("lever_score_GW")
        )
        .sort("lever_score_GW", descending=True)
    )
    both.write_csv(TABLES / "q5_corridor_leverage.csv")

    print("=== Nouveaux corridors (0 existant -> planifié) ===")
    print(
        new_corridors.select(["from_iso3", "to_iso3", "planned_mw", "multiplier"])
        .head(20)
        .to_pandas()
        .to_string(index=False)
    )
    print("\n=== Top corridors planifiés Europe ===")
    print(
        pivot.select(["from_iso3", "to_iso3", "existing_mw", "planned_mw", "multiplier"])
        .head(15)
        .to_pandas()
        .to_string(index=False)
    )
    print("\n=== Q5 : levier (top 10) ===")
    print(
        both.select(
            ["from_iso3", "to_iso3", "existing_mw", "planned_mw", "curtail_risk", "lever_score_GW"]
        )
        .head(10)
        .to_pandas()
        .to_string(index=False)
    )


if __name__ == "__main__":
    main()
