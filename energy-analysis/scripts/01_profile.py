#!/usr/bin/env python
"""Profil du parc européen + biais de couverture (questions 1 & 2).

Q1 : quelles données la carte ne cartographie-t-elle pas ?
Q2 : quelle est la "résolution temporelle" de chaque couche ?
"""

from __future__ import annotations

import polars as pl

from energyeu.config import (
    ACTIVE_STATUSES,
    DEAD_STATUSES,
    DISPATCHABLE_TYPES,
    PIPELINE_STATUSES,
    VARIABLE_TYPES,
)
from energyeu.load import OUT_DIR, ensure_processed, europe_grid

TABLES = OUT_DIR / "tables"
FIGURES = OUT_DIR / "figures"
TABLES.mkdir(parents=True, exist_ok=True)
FIGURES.mkdir(parents=True, exist_ok=True)


def main() -> None:
    df = ensure_processed(force=True)
    eur = europe_grid(df)

    # ------------------------------------------------------------------
    # 1) Parc opérationnel européen par type et statut
    # ------------------------------------------------------------------
    op = eur.filter(pl.col("status").is_in(ACTIVE_STATUSES))
    by_type = (
        op.group_by("type")
        .agg(
            pl.len().alias("n_units"),
            pl.col("capacity_mw").sum().alias("capacity_mw"),
        )
        .sort("capacity_mw", descending=True)
    )
    by_type.write_csv(TABLES / "operating_by_type_europe.csv")

    by_status = (
        eur.group_by("status")
        .agg(pl.len().alias("n_units"), pl.col("capacity_mw").sum().alias("capacity_mw"))
        .sort("capacity_mw", descending=True)
    )
    by_status.write_csv(TABLES / "status_europe.csv")

    by_country = (
        op.group_by("iso3")
        .agg(
            pl.len().alias("n_units"),
            pl.col("capacity_mw").sum().alias("capacity_mw"),
        )
        .sort("capacity_mw", descending=True)
    )
    by_country.write_csv(TABLES / "operating_by_country_europe.csv")

    # Mix par pays (part renouvelable variable vs dispatchable)
    mix = (
        op.with_columns(
            pl.when(pl.col("type").is_in(DISPATCHABLE_TYPES))
            .then(pl.lit("dispatchable"))
            .when(pl.col("type").is_in(VARIABLE_TYPES))
            .then(pl.lit("variable"))
            .otherwise(pl.lit("other"))
            .alias("class")
        )
        .group_by("iso3", "class")
        .agg(pl.col("capacity_mw").sum().alias("capacity_mw"))
        .pivot(on="class", index="iso3", values="capacity_mw", aggregate_function="sum")
        .fill_null(0.0)
        .with_columns(
            (pl.col("variable") / (pl.col("dispatchable") + pl.col("variable") + 1e-9))
            .alias("var_share")
        )
    )
    mix.write_csv(TABLES / "mix_by_country_europe.csv")

    # ------------------------------------------------------------------
    # Q1 — CE QUI N'EST PAS CARTOGRAPHIÉ
    # ------------------------------------------------------------------
    n = eur.height
    no_coord = eur.filter(pl.col("Latitude").is_null() | pl.col("Longitude").is_null())
    no_cap = eur.filter(pl.col("capacity_mw").is_null())
    no_owner = eur.filter(pl.col("Owner(s)").is_null())
    no_start = eur.filter(pl.col("year").is_null())
    missing = pl.DataFrame(
        {
            "indicator": [
                "units_without_coordinates",
                "units_without_capacity",
                "units_without_owner",
                "units_without_start_year",
                "units_without_operator",
                "units_without_gem_id",
            ],
            "n": [
                no_coord.height,
                no_cap.height,
                no_owner.height,
                no_start.height,
                eur.filter(pl.col("Operator(s)").is_null()).height,
                eur.filter(pl.col("GEM unit/phase ID").is_null()).height,
            ],
            "share": [
                no_coord.height / n,
                no_cap.height / n,
                no_owner.height / n,
                no_start.height / n,
                eur.filter(pl.col("Operator(s)").is_null()).height / n,
                eur.filter(pl.col("GEM unit/phase ID").is_null()).height / n,
            ],
        }
    )
    missing.write_csv(TABLES / "q1_missingness_europe.csv")

    # Biais par type : part des unités sans coordonnées
    by_type_missing = (
        eur.group_by("type")
        .agg(
            pl.len().alias("n"),
            pl.col("Latitude").is_null().mean().alias("share_no_coord"),
            pl.col("capacity_mw").is_null().mean().alias("share_no_capacity"),
            pl.col("Owner(s)").is_null().mean().alias("share_no_owner"),
            pl.col("year").is_null().mean().alias("share_no_startyear"),
        )
        .sort("n", descending=True)
    )
    by_type_missing.write_csv(TABLES / "q1_missingness_by_type.csv")

    # Biais par pays : top pays avec capacité sans coordonnées
    no_coord_cap = (
        no_coord.group_by("iso3")
        .agg(pl.col("capacity_mw").sum().alias("cap_no_coord_mw"))
        .sort("cap_no_coord_mw", descending=True)
        .head(15)
    )
    no_coord_cap.write_csv(TABLES / "q1_no_coord_by_country.csv")

    # « other » et « unknown » dans les champs technologiques
    otherish = eur.filter(
        pl.col("Technology").str.contains("unknown|other", literal=False)
        | pl.col("Fuel (combustion only)").str.contains("unknown|other", literal=False)
    )
    other_by_type = (
        otherish.group_by("type", "status")
        .agg(pl.len().alias("n"), pl.col("capacity_mw").sum().alias("capacity_mw"))
        .sort("capacity_mw", descending=True)
    )
    other_by_type.write_csv(TABLES / "q1_other_unknown_units.csv")

    # ------------------------------------------------------------------
    # Q2 — RÉSOLUTION TEMPORELLE / VINTAGES
    # ------------------------------------------------------------------
    vintage = (
        eur.filter(pl.col("year").is_not_null())
        .with_columns((pl.col("year").clip(1970, 2026)).alias("year"))
        .group_by("year", "type")
        .agg(pl.col("capacity_mw").sum().alias("capacity_mw"))
        .sort("year", "type")
    )
    vintage.write_csv(TABLES / "q2_vintage_by_year_type.csv")

    # Années de départ manquantes par statut (illusion de "nouveau parc")
    start_missing_status = (
        eur.group_by("status")
        .agg(
            pl.len().alias("n"),
            pl.col("year").is_null().mean().alias("share_no_start"),
        )
        .sort("n", descending=True)
    )
    start_missing_status.write_csv(TABLES / "q2_no_startyear_by_status.csv")

    # Capacité opérationnelle dont on ne connaît pas l'année (stock sans date)
    q2_stock_no_date = pl.DataFrame(
        {
            "scope": ["europe_grid"],
            "op_cap_mw": [op["capacity_mw"].sum()],
            "op_cap_no_startyear_mw": [
                op.filter(pl.col("year").is_null())["capacity_mw"].sum()
            ],
            "pipeline_cap_mw": [
                eur.filter(pl.col("status").is_in(PIPELINE_STATUSES))["capacity_mw"].sum()
            ],
            "dead_cap_mw": [
                eur.filter(pl.col("status").is_in(DEAD_STATUSES))["capacity_mw"].sum()
            ],
        }
    )
    q2_stock_no_date.write_csv(TABLES / "q2_stock_flows.csv")

    # ------------------------------------------------------------------
    # Print résumé
    # ------------------------------------------------------------------
    print("=== Europe (grille) : unités totales =", eur.height)
    print("\nOpérationnel par type (GW):")
    print(by_type.with_columns((pl.col("capacity_mw") / 1e3).round(1)).to_pandas().to_string(index=False))
    print("\nQ1 - part d'unités sans coordonnées par type:")
    print(
        by_type_missing.select(["type", "n", "share_no_coord"])
        .with_columns((pl.col("share_no_coord") * 100).round(1).alias("pct"))
        .to_pandas()
        .to_string(index=False)
    )
    print("\nQ2 - stock/flux (GW):")
    print(
        q2_stock_no_date.with_columns(
            (pl.col("op_cap_mw") / 1e3).round(1).alias("op_GW"),
            (pl.col("pipeline_cap_mw") / 1e3).round(1).alias("pipeline_GW"),
            (pl.col("dead_cap_mw") / 1e3).round(1).alias("dead_GW"),
        )
        .select(["scope", "op_GW", "pipeline_GW", "dead_GW"])
        .to_pandas()
        .to_string(index=False)
    )


if __name__ == "__main__":
    main()
