#!/usr/bin/env python
"""Questions 3, 6, 11, 12, 13 — analyses complémentaires.

Q3  : retirer bioénergie/geothermal/unknown — quels nœuds deviennent insolables ?
Q6  : ombres de prix — proxy de congestion (renouvelable par MW d'interconnexion)
Q11 : couplage fossile caché par TWh renouvelable (ratio de capacité)
Q12 : paires de pays en friction (divergence de mix × capacité d'interconnexion)
Q13 : zombies et projets financés non construits
"""

from __future__ import annotations

import polars as pl

from energyeu.config import (
    ACTIVE_STATUSES,
    EUROPE_GRID,
    FOSSIL_TYPES,
    NORTH_AFRICA,
    PIPELINE_STATUSES,
    STALLED_STATUSES,
)
from energyeu.load import OUT_DIR, ensure_processed, europe_grid, load_gtd
from energyeu.peak import PEAK_DEMAND_GW

TABLES = OUT_DIR / "tables"
TABLES.mkdir(parents=True, exist_ok=True)

SCOPE = EUROPE_GRID | NORTH_AFRICA


def main() -> None:
    df = ensure_processed()
    eur = europe_grid(df)
    op = eur.filter(pl.col("status").is_in(ACTIVE_STATUSES))

    # ------------------------------------------------------------------
    # Q3 — retrait bio/geo/unknown
    # ------------------------------------------------------------------
    cap = (
        op.group_by("iso3", "type")
        .agg(pl.col("capacity_mw").sum().alias("mw"))
        .pivot(on="type", index="iso3", values="mw", aggregate_function="sum")
        .fill_null(0.0)
    )
    rows = []
    for r in cap.iter_rows(named=True):
        iso = r["iso3"]
        peak = PEAK_DEMAND_GW.get(iso, 0.0)
        fossil = r.get("oil/gas", 0.0) + r.get("coal", 0.0)
        nuclear = r.get("nuclear", 0.0)
        hydro = r.get("hydropower", 0.0)
        bio_geo = r.get("bioenergy", 0.0) + r.get("geothermal", 0.0)
        dispatch_all = fossil + nuclear + hydro + bio_geo
        dispatch_no_bio = fossil + nuclear + hydro
        rows.append({
            "iso3": iso,
            "peak_gw": peak,
            "bio_geo_mw": bio_geo,
            "bio_geo_share_dispatch": bio_geo / (dispatch_all + 1e-6),
            "dispatch_all_mw": dispatch_all,
            "dispatch_no_bio_mw": dispatch_no_bio,
            "cover_ratio_after": (dispatch_no_bio / 1e3) / (0.9 * peak + 1e-6),
            "insolvable": ((dispatch_no_bio / 1e3) < 0.5 * 0.9 * peak),
        })
    q3 = pl.DataFrame(rows).sort("bio_geo_share_dispatch", descending=True)
    q3.write_csv(TABLES / "q3_bio_other_removal.csv")

    # ------------------------------------------------------------------
    # Q6 — ombres de prix : renouvelable par MW d'interconnexion
    # ------------------------------------------------------------------
    topo = pl.read_csv(TABLES / "topology_countries.csv")
    ren = (
        op.filter(pl.col("type").is_in(["wind", "utility-scale solar", "hydropower"]))
        .group_by("iso3")
        .agg(pl.col("capacity_mw").sum().alias("ren_op_mw"))
    )
    q6 = (
        ren.join(topo.select(["iso3", "inter_mw"]), on="iso3", how="left")
        .fill_null(0.0)
        .with_columns(
            pl.when(pl.col("inter_mw") > 0.0)
            .then((pl.col("ren_op_mw") / 1e3) / (pl.col("inter_mw") / 1e3))
            .otherwise(None)
            .alias("ren_gw_per_inter_gw")
        )
        .sort("ren_gw_per_inter_gw", descending=True, nulls_last=True)
    )
    q6.write_csv(TABLES / "q6_shadow_prices_proxy.csv")

    # ------------------------------------------------------------------
    # Q11 — couplage fossile par GW renouvelable
    # ------------------------------------------------------------------
    fossil = (
        op.filter(pl.col("type").is_in(FOSSIL_TYPES))
        .group_by("iso3").agg(pl.col("capacity_mw").sum().alias("fossil_op_mw"))
    )
    q11 = (
        ren.join(fossil, on="iso3", how="left").fill_null(0.0)
        .with_columns((pl.col("fossil_op_mw") / (pl.col("ren_op_mw") + 1e-6)).alias("fossil_per_ren"))
        .sort("fossil_per_ren", descending=True)
    )
    q11.write_csv(TABLES / "q11_fossil_renewable_coupling.csv")

    # ------------------------------------------------------------------
    # Q12 — paires en friction (divergence de mix × capacité)
    # ------------------------------------------------------------------
    gtd = load_gtd()
    mix_share = (
        op.group_by("iso3")
        .agg(
            (pl.col("capacity_mw").filter(pl.col("type").is_in(FOSSIL_TYPES)).sum()
             / pl.col("capacity_mw").sum()).alias("fossil_share")
        )
    )
    lines = (
        gtd.group_by(["from_iso3", "to_iso3"])
        .agg(pl.col("max_flow_mw").sum().alias("inter_mw"))
        .filter(pl.col("from_iso3").is_in(SCOPE) & pl.col("to_iso3").is_in(SCOPE))
    )
    q12 = (
        lines.join(mix_share.rename({"iso3": "from_iso3", "fossil_share": "fossil_a"}),
                   on="from_iso3", how="left")
        .join(mix_share.rename({"iso3": "to_iso3", "fossil_share": "fossil_b"}),
              on="to_iso3", how="left")
        .filter(pl.col("fossil_a").is_not_null() & pl.col("fossil_b").is_not_null())
        .with_columns(
            pl.col("fossil_a").fill_null(0.0),
            pl.col("fossil_b").fill_null(0.0),
        )
        .with_columns(
            ((pl.col("fossil_a") - pl.col("fossil_b")).abs().alias("mix_divergence")),
        )
        .with_columns(
            (pl.col("mix_divergence") * pl.col("inter_mw") / 1e3).alias("friction_score_GW")
        )
        .sort("friction_score_GW", descending=True)
    )
    q12.write_csv(TABLES / "q12_friction_pairs.csv")

    # ------------------------------------------------------------------
    # Q13 — zombies et pipeline
    # ------------------------------------------------------------------
    # Zombies : annoncés/shelved (jamais construits), gros projets sans capital
    # identifiable (pas d'owner) — proxy "approuvés mais sans cash".
    zombies = (
        eur.filter(pl.col("status").is_in(STALLED_STATUSES))
        .group_by("iso3", "type", "status")
        .agg(pl.len().alias("n"), pl.col("capacity_mw").sum().alias("capacity_mw"))
        .sort("capacity_mw", descending=True)
    )
    zombies.write_csv(TABLES / "q13_zombies.csv")

    pipeline_no_owner = (
        eur.filter(pl.col("status").is_in(PIPELINE_STATUSES))
        .group_by("iso3")
        .agg(
            pl.col("capacity_mw").sum().alias("pipeline_mw"),
            pl.col("capacity_mw").filter(pl.col("Owner(s)").is_null()).sum().alias("pipeline_no_owner_mw"),
            pl.len().alias("n"),
            pl.col("Owner(s)").is_null().mean().alias("share_no_owner"),
        )
        .sort("pipeline_mw", descending=True)
    )
    pipeline_no_owner.write_csv(TABLES / "q13_pipeline_no_owner.csv")

    construction = (
        eur.filter(pl.col("status") == "construction")
        .group_by("iso3", "type")
        .agg(pl.len().alias("n"), pl.col("capacity_mw").sum().alias("capacity_mw"))
        .sort("capacity_mw", descending=True)
    )
    construction.write_csv(TABLES / "q13_construction_pipeline.csv")

    # ------------------------------------------------------------------
    # Sorties
    # ------------------------------------------------------------------
    print("=== Q3 — nœuds insolvables après retrait bio/geo/unknown ===")
    print(q3.filter(pl.col("insolvable")).select(
        ["iso3", "peak_gw", "bio_geo_mw", "bio_geo_share_dispatch", "dispatch_no_bio_mw", "cover_ratio_after"]
    ).to_pandas().to_string(index=False))
    print("\n=== Q3 — pays les plus dépendants de bio/geo (top 10) ===")
    print(q3.select(["iso3", "bio_geo_mw", "bio_geo_share_dispatch", "dispatch_all_mw"])
          .head(10).to_pandas().to_string(index=False))
    print("\n=== Q6 — ombres de prix (ren GW / interconnexion GW, top 12) ===")
    print(q6.head(12).to_pandas().to_string(index=False))
    print("\n=== Q11 — fossile par GW renouvelable (top 12) ===")
    print(q11.select(["iso3", "ren_op_mw", "fossil_op_mw", "fossil_per_ren"])
          .head(12).to_pandas().to_string(index=False))
    print("\n=== Q12 — friction (top 12) ===")
    print(q12.select(["from_iso3", "to_iso3", "inter_mw", "fossil_a", "fossil_b", "friction_score_GW"])
          .head(12).to_pandas().to_string(index=False))
    print("\n=== Q13 — zombies (5 ans+ annoncés, top 12) ===")
    print(zombies.head(12).to_pandas().to_string(index=False))
    print("\nEn construction (GW par type) :")
    print(construction.with_columns((pl.col("capacity_mw")/1e3).round(2).alias("GW"))
          .group_by("type").agg(pl.col("GW").sum()).sort("GW", descending=True)
          .to_pandas().to_string(index=False))


if __name__ == "__main__":
    main()
