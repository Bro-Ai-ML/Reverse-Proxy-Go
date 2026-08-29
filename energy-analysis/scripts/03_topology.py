#!/usr/bin/env python
"""Topologie du réseau d'interconnexion européen (question 4).

Objectif : identifier les "ponts thermiques" — pays dont l'effacement (ou la
fermeture de leurs centrales fossiles) isole des clusters du reste du réseau.
"""

from __future__ import annotations

from typing import Any

import networkx as nx
import polars as pl

from energyeu.config import (
    ACTIVE_STATUSES,
    EUROPE_GRID,
    FOSSIL_TYPES,
    NORTH_AFRICA,
    SILK_ROAD,
)
from energyeu.load import OUT_DIR, ensure_processed, load_gtd

TABLES = OUT_DIR / "tables"
TABLES.mkdir(parents=True, exist_ok=True)

SCOPE = EUROPE_GRID | NORTH_AFRICA | SILK_ROAD


def build_graph(gtd: pl.DataFrame, kind: str = "existing") -> nx.Graph:
    """Graphe non-orienté des interconnexions (poids = capacité MW)."""
    g = nx.Graph()
    sub = gtd.filter((pl.col("kind") == kind) & (pl.col("max_flow_mw") > 0.0))
    for row in sub.iter_rows(named=True):
        a, b, w = row["from_iso3"], row["to_iso3"], float(row["max_flow_mw"])
        if a in SCOPE and b in SCOPE and a != b:
            if g.has_edge(a, b):
                g[a][b]["weight"] += w
            else:
                g.add_edge(a, b, weight=w)
    return g


def main() -> None:
    gtd = load_gtd()
    df = ensure_processed()
    op = df.filter(pl.col("status").is_in(ACTIVE_STATUSES))

    # Attributs par pays : capacités fossile / renouvelable / totale
    cap = (
        op.group_by("iso3")
        .agg(
            pl.col("capacity_mw").sum().alias("total_mw"),
            pl.col("capacity_mw")
            .filter(pl.col("type").is_in(FOSSIL_TYPES))
            .sum()
            .alias("fossil_mw"),
            pl.col("capacity_mw")
            .filter(pl.col("type").is_in(["wind", "utility-scale solar", "hydropower"]))
            .sum()
            .alias("ren_mw"),
        )
        .filter(pl.col("iso3").is_in(SCOPE))
    )
    cap_attr = {r["iso3"]: (r["total_mw"], r["fossil_mw"], r["ren_mw"]) for r in cap.iter_rows(named=True)}

    g = build_graph(gtd, "existing")
    g2 = build_graph(gtd, "planned")

    # ------------------------------------------------------------------
    # Degrés et capacité d'interconnexion par pays
    # ------------------------------------------------------------------
    rows = []
    for n in g.nodes():
        total, fossil, ren = cap_attr.get(n, (0.0, 0.0, 0.0))
        inter = sum(w for _, _, w in g.edges(n, data="weight"))
        inter_planned = sum(w for _, _, w in g2.edges(n, data="weight"))
        rows.append({
            "iso3": n,
            "degree": g.degree(n),
            "inter_mw": inter,
            "inter_planned_mw": inter_planned,
            "total_mw": total,
            "fossil_mw": fossil,
            "ren_mw": ren,
            "inter_ratio": inter / (total + 1.0),
            "is_articulation": n in nx.articulation_points(g) if g.degree(n) > 1 else False,
        })
    topo = pl.DataFrame(rows).sort("inter_mw", descending=True)
    topo.write_csv(TABLES / "topology_countries.csv")

    # Îles énergétiques : degré 1 ou interconnexion très faible
    islands = topo.filter((pl.col("degree") <= 1) | (pl.col("inter_ratio") < 0.1))
    islands.write_csv(TABLES / "q4_islands.csv")

    # ------------------------------------------------------------------
    # Q4 — pivots topologiques : articulation + fossile dominant
    # ------------------------------------------------------------------
    arts = [
        n for n in g.nodes() if g.degree(n) > 1 and n in nx.articulation_points(g)
    ]
    # taille du composant le plus grand après retrait du nœud
    pivot_rows = []
    for n in arts:
        g_tmp = g.copy()
        g_tmp.remove_node(n)
        comp_sizes = [len(c) for c in nx.connected_components(g_tmp)]  # type: ignore[arg-type]
        comps = sorted(comp_sizes, reverse=True)
        largest = comps[0] if comps else 0
        n_comp = len(comps)
        total, fossil, ren = cap_attr.get(n, (0.0, 0.0, 0.0))
        pivot_rows.append({
            "iso3": n,
            "fossil_mw": fossil,
            "total_mw": total,
            "fossil_share": fossil / (total + 1.0),
            "n_components_after": n_comp,
            "largest_component_size_after": largest,
            "nodes_isolated_share": (g.number_of_nodes() - largest) / g.number_of_nodes(),
        })
    pivots = pl.DataFrame(pivot_rows).sort("fossil_share", descending=True)
    pivots.write_csv(TABLES / "q4_pivot_nodes.csv")

    # Edge betweenness (interconnexions critiques)
    eb = nx.edge_betweenness_centrality(g, weight="weight")
    edge_rows = [
        {"from_iso3": a, "to_iso3": b, "betweenness": v, "capacity_mw": g[a][b]["weight"]}
        for (a, b), v in eb.items()
    ]
    edge_tb = pl.DataFrame(edge_rows).sort("betweenness", descending=True)
    edge_tb.write_csv(TABLES / "q4_critical_edges.csv")

    # ------------------------------------------------------------------
    # Composantes de la grille Europe après retrait des hubs fossiles
    # ------------------------------------------------------------------
    eur_sub = nx.subgraph(g, [n for n in g.nodes() if n in EUROPE_GRID])
    # les stubs networkx de cette version typent mal connected_components
    comps_any: list[Any] = sorted(
        nx.connected_components(eur_sub), key=lambda c: len(c), reverse=True
    )
    comp_rows = [
        {"component": i, "n_countries": len(c), "countries": ",".join(sorted(c))}
        for i, c in enumerate(comps_any)
    ]
    pl.DataFrame(comp_rows).write_csv(TABLES / "q4_components_europe.csv")

    # ------------------------------------------------------------------
    # Q4 — clusters renouvelables faiblement rattachés au backbone fossile
    # ------------------------------------------------------------------
    # Graphe restreint aux pays à forte part renouvelable (> 55 % de leur parc)
    ren_share = {
        n: (ren / (total + 1.0))
        for n, (total, fossil, ren) in cap_attr.items()
    }
    ren_cluster_nodes = {n for n in g.nodes() if ren_share.get(n, 0.0) > 0.55}
    ren_g = nx.subgraph(g, ren_cluster_nodes)
    ren_comps = sorted(nx.connected_components(ren_g), key=len, reverse=True)

    cluster_rows = []
    for c in ren_comps:
        # liens de ce cluster vers le reste du réseau
        ext_links = []
        for n in c:
            for nb, w in g[n].items():
                if nb not in c:
                    ext_links.append((n, nb, w.get("weight", 0.0)))
        total_ext = sum(w for _, _, w in ext_links)
        cluster_rows.append({
            "cluster": ",".join(sorted(c)),
            "n_countries": len(c),
            "ren_op_gw": sum(cap_attr[n][2] for n in c) / 1e3,
            "n_external_links": len(ext_links),
            "external_capacity_mw": total_ext,
            "links": ";".join(f"{a}->{b}:{w:.0f}" for a, b, w in ext_links),
        })
    if cluster_rows:
        pl.DataFrame(cluster_rows).sort("ren_op_gw", descending=True).write_csv(
            TABLES / "q4_renewable_clusters.csv"
        )
        print("\n=== Clusters renouvelables (>55% ren) et leur rattachement ===")
        for r in cluster_rows:
            print(
                f"  {r['cluster']} : {r['ren_op_gw']:.0f} GW ren, "
                f"{r['n_external_links']} liens externes, {r['external_capacity_mw']:.0f} MW"
            )
    else:
        print("\n(no renewable cluster >55%)")

    # ------------------------------------------------------------------
    # Sortie
    # ------------------------------------------------------------------
    print("=== Îles énergétiques (degré<=1 ou inter_ratio<10%) ===")
    print(
        islands.select(["iso3", "degree", "inter_mw", "total_mw", "inter_ratio", "fossil_mw"])
        .to_pandas()
        .to_string(index=False)
    )
    print("\n=== Pivots topologiques (points d'articulation) avec part fossile ===")
    print(
        pivots.select(["iso3", "fossil_mw", "fossil_share", "n_components_after", "nodes_isolated_share"])
        .head(12)
        .to_pandas()
        .to_string(index=False)
    )
    print("\n=== Interconnexions critiques (top betweenness) ===")
    print(edge_tb.head(12).to_pandas().to_string(index=False))
    print(f"\n=== Composantes grille Europe existante : {len(comps_any)} ===")
    for i, c in enumerate(comps_any):
        print(f"  comp{i}: {len(c)} pays -> {sorted(c)[:8]}{'...' if len(c) > 8 else ''}")


if __name__ == "__main__":
    main()
