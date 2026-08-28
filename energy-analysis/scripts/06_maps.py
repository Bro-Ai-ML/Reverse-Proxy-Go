#!/usr/bin/env python
"""Visualisations : carte des centrales, corridors planifiés, duck curve,
croisement bayésien stockage/gaz."""

from __future__ import annotations

import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt  # noqa: E402
import numpy as np  # noqa: E402
import polars as pl  # noqa: E402
from matplotlib.axes import Axes  # noqa: E402

from energyeu.config import ACTIVE_STATUSES  # noqa: E402
from energyeu.load import OUT_DIR, ensure_processed, europe_grid, load_gtd  # noqa: E402

RAW = Path(__file__).resolve().parents[1] / "data" / "raw"
FIGURES = OUT_DIR / "figures"
FIGURES.mkdir(parents=True, exist_ok=True)

TYPE_COLORS = {
    "utility-scale solar": "#f2b134",
    "wind": "#4da6ff",
    "hydropower": "#2e8b57",
    "oil/gas": "#8b0000",
    "coal": "#2f2f2f",
    "nuclear": "#b44de0",
    "bioenergy": "#7cb342",
    "geothermal": "#e57373",
}


def load_country_geoms() -> list[tuple[str, list]]:
    """Charge les géométries pays (iso3 -> listes de polygones lat/lon)."""
    with open(RAW / "countries.geojson") as f:
        d = json.load(f)
    out = []
    for feat in d["features"]:
        iso = feat["properties"].get("ISO_A3")
        geom = feat.get("geometry")
        if not iso or not geom:
            continue
        polys: list = []
        if geom["type"] == "Polygon":
            polys.append(geom["coordinates"])
        else:
            polys.extend(geom["coordinates"])
        out.append((iso, polys))
    return out


def draw_borders(ax: Axes) -> None:
    for _iso, polys in load_country_geoms():
        for poly in polys:
            for ring in poly:
                xs = [p[0] for p in ring]
                ys = [p[1] for p in ring]
                ax.plot(xs, ys, color="#bbbbbb", lw=0.3, zorder=1)


def map_europe_plants() -> None:
    df = ensure_processed()
    op = europe_grid(df).filter(pl.col("status").is_in(ACTIVE_STATUSES))
    fig, ax = plt.subplots(figsize=(14, 10))
    draw_borders(ax)
    # échantillonner pour lisibilité (les plus grosses unités d'abord)
    op = op.sort("capacity_mw", descending=True).head(6000)
    for t, color in TYPE_COLORS.items():
        sub = op.filter(pl.col("type") == t)
        if sub.height == 0:
            continue
        size = np.clip(sub["capacity_mw"].to_numpy() / 100.0, 2, 60)
        ax.scatter(sub["Longitude"].to_numpy(), sub["Latitude"].to_numpy(),
                   s=size, c=color, alpha=0.55, label=f"{t} ({sub.height})", linewidths=0)
    ax.set_xlim(-12, 45)
    ax.set_ylim(34, 72)
    ax.set_title("Centrales européennes (GIPT août 2026) — top 6 000 unités par capacité")
    ax.legend(loc="lower left", fontsize=8, framealpha=0.9)
    ax.set_xlabel("Longitude")
    ax.set_ylabel("Latitude")
    fig.tight_layout()
    fig.savefig(FIGURES / "europe_plants.png", dpi=130)
    plt.close(fig)
    print("saved europe_plants.png")


def map_corridors() -> None:
    """Carte des corridors planifiés (GTD planned) impliquant l'Europe."""
    gtd = load_gtd()
    nodes = pl.read_csv(RAW / "nodes.csv", null_values="")
    centers = {r["iso"]: (r["Pop_Lon"], r["Pop_Lat"]) for r in nodes.iter_rows(named=True)}

    planned = (
        gtd.filter(pl.col("kind") == "planned")
        .filter(pl.col("max_flow_mw") > 0.0)
        .group_by(["from_iso3", "to_iso3"])
        .agg(pl.col("max_flow_mw").sum().alias("mw"))
        .sort("mw", descending=True)
        .head(40)
    )

    fig, ax = plt.subplots(figsize=(14, 10))
    draw_borders(ax)

    for r in planned.iter_rows(named=True):
        a, b = r["from_iso3"], r["to_iso3"]
        if a not in centers or b not in centers:
            continue
        (x1, y1), (x2, y2) = centers[a], centers[b]
        w = np.clip(r["mw"] / 500.0, 1, 14)
        ax.annotate(
            "", xy=(x2, y2), xytext=(x1, y1),
            arrowprops=dict(arrowstyle="-|>", color="#d62728", lw=w, alpha=0.6,
                            shrinkA=0, shrinkB=0, mutation_scale=12),
        )
        ax.text((x1 + x2) / 2, (y1 + y2) / 2, f"{r['mw']/1e3:.1f} GW",
                fontsize=6, ha="center", va="center", color="#7a0f0f", zorder=5)

    ax.set_xlim(-12, 60)
    ax.set_ylim(25, 72)
    ax.set_title("Corridors d'interconnexion planifiés (GTD v1.0) — capacité en GW")
    fig.tight_layout()
    fig.savefig(FIGURES / "corridors_planned.png", dpi=130)
    plt.close(fig)
    print("saved corridors_planned.png")


def duck_curve_proxy() -> None:
    """Proxy statique de duck curve pour 4 pays (DE, ES, IT, GR)."""
    q8 = pl.read_csv(OUT_DIR / "tables" / "q8_evening_fossil.csv")
    fig, axes = plt.subplots(2, 2, figsize=(12, 8), sharex=True)
    hour = np.arange(0, 24)
    # forme de charge normalisée (hiver) et production solaire ciel clair
    demand = 0.78 + 0.16 * np.sin((hour - 8) / 24 * 2 * np.pi) ** 2 + 0.06 * np.cos((hour - 20) / 24 * 2 * np.pi)
    demand = demand / demand.max()
    solar_shape = np.clip(np.sin((hour - 7) / 12 * np.pi), 0, 1) * np.clip(np.sin((hour - 7.5) / 11.5 * np.pi), 0, 1)
    solar_shape = solar_shape / solar_shape.max()

    for ax, iso in zip(axes.ravel(), ["DEU", "ESP", "ITA", "GRC"], strict=True):
        r = q8.filter(pl.col("iso3") == iso).to_pandas().iloc[0]
        peak = r["peak_gw"]
        solar_gw = r["solar_gw"]
        # net load proxy : charge − production solaire (en GW)
        net = demand * peak - solar_shape * solar_gw * 0.9
        ax.plot(hour, demand * peak, label="charge (proxy)", color="#444")
        ax.plot(hour, solar_shape * solar_gw * 0.9, label="solaire (proxy)", color="#f2b134")
        ax.plot(hour, net, label="charge nette (proxy)", color="#d62728", lw=2)
        ax.axhline(peak * 0.9, color="#888", ls=":", lw=0.8)
        ax.fill_between([17, 21], 0, peak * 1.05, color="#8b0000", alpha=0.08)
        ax.set_title(f"{iso} — pointe du soir (bande rouge) : index fossile {r['evening_fossil_index']:.2f}")
        ax.legend(fontsize=7, loc="upper right")
        ax.set_ylim(0, peak * 1.05)
    for ax in axes[-1]:
        ax.set_xlabel("heure")
    fig.suptitle("Proxy duck curve — GW (hiver, ciel clair)", fontsize=12)
    fig.tight_layout()
    fig.savefig(FIGURES / "duck_curve_proxy.png", dpi=130)
    plt.close(fig)
    print("saved duck_curve_proxy.png")


def bayesian_crossover_figure() -> None:
    res = pl.read_csv(OUT_DIR / "tables" / "q7_bayesian_crossover.csv")
    fig, ax = plt.subplots(figsize=(9, 5))
    ax.plot(res["year"], res["stor_gw_med"], color="#2e8b57", lw=2, label="stockage GEM-tracked (médiane)")
    ax.fill_between(res["year"], res["stor_gw_p10"], res["stor_gw_p90"], color="#2e8b57", alpha=0.25,
                    label="intervalle 10-90 %")
    ax.plot(res["year"], res["gas_gw_traj"], color="#8b0000", lw=2, ls="--", label="gaz (déclin 2 %/an)")
    ax.set_xlabel("année")
    ax.set_ylabel("GW")
    ax.set_title("Q7 — croisement stockage vs gaz (modèle bayésien pyro)")
    ax.legend()
    fig.tight_layout()
    fig.savefig(FIGURES / "q7_bayesian_crossover.png", dpi=130)
    plt.close(fig)
    print("saved q7_bayesian_crossover.png")


def main() -> None:
    map_europe_plants()
    map_corridors()
    duck_curve_proxy()
    bayesian_crossover_figure()


if __name__ == "__main__":
    main()
