#!/usr/bin/env python
"""Modèles statistiques (questions 9, 10, 14, 15 + croisement bayésien Q7).

- Q7 : modèle bayésien pyro de croissance du stockage vs gaz
- Q9 : indice de cascade canicule/sécheresse (scipy, copule gaussienne)
- Q10 : clustering sklearn des zones "hubris" (renouvelables sans inertie)
- Q14 : proxy de croissance innovation vs capacité brute
- Q15 : entropie de diversité et résilience
"""

from __future__ import annotations

import numpy as np
import polars as pl
from scipy import stats
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler

from energyeu.config import (
    ACTIVE_STATUSES,
    EUROPE_GRID,
    FOSSIL_TYPES,
    NORTH_AFRICA,
    PIPELINE_STATUSES,
    VARIABLE_TYPES,
)
from energyeu.load import OUT_DIR, ensure_processed, europe_grid

TABLES = OUT_DIR / "tables"
TABLES.mkdir(parents=True, exist_ok=True)

SCOPE = EUROPE_GRID | NORTH_AFRICA


def load_country_features() -> pl.DataFrame:
    """Construit la matrice de caractéristiques par pays."""
    df = ensure_processed()
    op = df.filter(pl.col("status").is_in(ACTIVE_STATUSES))
    eur = europe_grid(df)

    cap = (
        op.group_by("iso3")
        .agg(
            pl.col("capacity_mw").sum().alias("total_mw"),
            pl.col("capacity_mw").filter(pl.col("type").is_in(FOSSIL_TYPES)).sum().alias("fossil_mw"),
            pl.col("capacity_mw").filter(pl.col("type").is_in(["nuclear"])).sum().alias("nuclear_mw"),
            pl.col("capacity_mw").filter(pl.col("type").is_in(["hydropower"])).sum().alias("hydro_mw"),
            pl.col("capacity_mw").filter(pl.col("type").is_in(["bioenergy", "geothermal"])).sum().alias("bio_geo_mw"),
            pl.col("capacity_mw").filter(pl.col("type").is_in(VARIABLE_TYPES)).sum().alias("var_mw"),
            pl.col("capacity_mw").filter(pl.col("type") == "utility-scale solar").sum().alias("solar_mw"),
            pl.col("capacity_mw").filter(pl.col("Technology").str.contains("pumped", literal=False)).sum().alias("pumped_mw"),
        )
        .filter(pl.col("iso3").is_in(SCOPE))
    )
    pipe = (
        eur.filter(pl.col("status").is_in(PIPELINE_STATUSES))
        .group_by("iso3")
        .agg(pl.col("capacity_mw").sum().alias("pipeline_mw"))
    )
    feat = cap.join(pipe, on="iso3", how="left").fill_null(0.0)
    return feat.with_columns(
        (pl.col("var_mw") / (pl.col("total_mw") + 1e-6)).alias("var_share"),
        (pl.col("fossil_mw") / (pl.col("total_mw") + 1e-6)).alias("fossil_share"),
        (pl.col("hydro_mw") / (pl.col("total_mw") + 1e-6)).alias("hydro_share"),
        (pl.col("pumped_mw") / (pl.col("total_mw") + 1e-6)).alias("storage_share"),
        (pl.col("pipeline_mw") / (pl.col("total_mw") + 1e-6)).alias("pipeline_ratio"),
    )


def q9_cascade(feat: pl.DataFrame) -> pl.DataFrame:
    """Q9 — indice de cascade (canicule + sécheresse).

    Hypothèses (transparentes) : risque = f(part hydro, part thermique,
    faible interconnexion, forte part variable). Probabilité conjointe de
    dépasser 1 écart-type sur chaque axe via une gaussienne multivariée.
    """
    cols = ["hydro_share", "fossil_share", "var_share"]
    X = feat.select(cols).to_numpy()
    Xz = stats.zscore(X, nan_policy="omit")
    Xz = np.nan_to_num(Xz, nan=0.0)
    mu = np.zeros(3)
    cov = np.cov(Xz, rowvar=False)
    mvn = stats.multivariate_normal(mean=mu, cov=cov + 1e-6 * np.eye(3))
    # P(chaque axe > 1 écart-type) : probabilité dans la queue conjointe
    tail = 1.0 - mvn.cdf(np.full(3, 1.0))
    inter = (
        pl.read_csv(TABLES / "topology_countries.csv")
        .select(["iso3", "inter_ratio"])
    )
    out = (
        feat.select(["iso3", "hydro_share", "fossil_share", "var_share"])
        .join(inter, on="iso3", how="left")
        .fill_null(0.0)
        .with_columns(
            # normalisation 0-1 de chaque dimension pour l'indice composite
            (pl.col("hydro_share").rank() / pl.len()).alias("r_hydro"),
            (pl.col("fossil_share").rank() / pl.len()).alias("r_fossil"),
            (pl.col("var_share").rank() / pl.len()).alias("r_var"),
            (-pl.col("inter_ratio").rank() / pl.len()).alias("r_nointer"),
            pl.lit(tail).alias("joint_tail_prob"),
        )
        .with_columns(
            (0.3 * pl.col("r_hydro") + 0.25 * pl.col("r_fossil")
             + 0.25 * pl.col("r_var") + 0.2 * pl.col("r_nointer")).alias("cascade_index")
        )
        .sort("cascade_index", descending=True)
    )
    out.write_csv(TABLES / "q9_cascade_index.csv")
    return out


def q10_hubris(feat: pl.DataFrame) -> pl.DataFrame:
    """Q10 — clusters 'hubris' : forte part variable, faible inertie, faible lien.

    KMeans sur les caractéristiques standardisées ; le cluster identifié
    'hubris' = part variable élevée + stockage faible + interconnexion faible.
    """
    X = feat.select(["var_share", "storage_share", "pipeline_ratio"]).to_numpy()
    X = np.nan_to_num(X, nan=0.0)
    Xs = StandardScaler().fit_transform(X)
    km = KMeans(n_clusters=4, n_init=20, random_state=42).fit(Xs)
    out = feat.with_columns(pl.Series("cluster", km.labels_.astype(int)))
    centroids = pd_centroids(km, ["var_share", "storage_share", "pipeline_ratio"])
    # cluster le plus "hubris" : centroïde var_share max
    hubris_k = int(np.argmax(centroids[:, 0]))
    out = out.with_columns((pl.col("cluster") == hubris_k).alias("is_hubris"))
    out.sort("var_share", descending=True).write_csv(TABLES / "q10_hubris_clusters.csv")
    print(f"  cluster hubris = {hubris_k} (centroïdes var_share: {centroids[:, 0].round(3)})")
    return out


def pd_centroids(km: KMeans, cols: list[str]) -> np.ndarray:
    """Renvoie les centroïdes du cluster dans l'espace original."""
    return np.asarray(km.cluster_centers_)


def q7_bayesian_crossover() -> None:
    """Q7 — croisement stockage vs gaz avec un modèle bayésien pyro.

    Modèle : log(C_stockage(t)) ~ N(a + b*(t-2000), sigma) sur la série
    GEM-tracked (pompage + solaire co-localisé, operating + construction).
    Prédiction jusqu'à l'année où C_stockage(t) >= C_gaz(t) (gaz en déclin
    linéaire à partir de 2026). Sortie : distribution de l'année de bascule.
    """
    import pyro  # noqa: PLC0415
    import pyro.distributions as dist  # noqa: PLC0415
    import torch  # noqa: PLC0415
    from pyro.contrib.autoguide import AutoDiagonalNormal  # noqa: PLC0415
    from pyro.infer import SVI, Trace_ELBO  # noqa: PLC0415
    from pyro.optim import Adam  # type: ignore[attr-defined]  # noqa: PLC0415

    stor = pl.read_csv(TABLES / "q7_storage_vintage.csv").sort("year")
    gas = pl.read_csv(TABLES / "q7_gas_vintage.csv").sort("year")

    x = torch.tensor(stor["year"].to_numpy(), dtype=torch.float64)
    y = torch.tensor(np.log(stor["mw_cum"].to_numpy() + 1.0), dtype=torch.float64)
    x_c = (x - 2000.0) / 10.0

    def model() -> None:
        a = pyro.sample("a", dist.Normal(2.0, 1.0))
        b = pyro.sample("b", dist.Normal(0.5, 0.5))
        sigma = pyro.sample("sigma", dist.LogNormal(0.0, 0.5))
        with pyro.plate("data", len(x_c)):
            pyro.sample("obs", dist.Normal(a + b * x_c, sigma), obs=y)

    guide = AutoDiagonalNormal(model)
    svi = SVI(model, guide, Adam({"lr": 0.05}), loss=Trace_ELBO())
    for _ in range(2000):
        svi.step()

    pred_years = np.arange(2026, 2041)
    xp = torch.tensor((pred_years - 2000.0) / 10.0, dtype=torch.float64)
    from pyro.infer import Predictive  # noqa: PLC0415

    predictive = Predictive(model, guide=guide, num_samples=500)
    samples = predictive()
    a_s = samples["a"].numpy().reshape(-1)
    b_s = samples["b"].numpy().reshape(-1)
    log_stor = a_s[:, None] + b_s[:, None] * xp.numpy()[None, :]
    stor_gw = np.exp(log_stor) / 1e3

    gas_gw_2026 = float(gas.filter(pl.col("year") == 2026)["mw_cum"].first() or 0.0) / 1e3  # type: ignore[arg-type]
    # déclin du gaz : retraits ~2 %/an à partir de 2026 (hypothèse prudente)
    gas_traj = gas_gw_2026 * np.exp(-0.02 * (pred_years - 2026))

    crossover = np.array([
        pred_years[idx[0]] if len(idx := np.where(s >= gas_traj)[0]) > 0 else 2040
        for s in stor_gw
    ])
    crossover_years = crossover[crossover < 2041]
    if len(crossover_years) == 0:
        crossover_years = np.array([2040])
    lo, med, hi = np.percentile(crossover_years, [10, 50, 90])
    res = pl.DataFrame({
        "year": pred_years,
        "stor_gw_med": np.median(stor_gw, axis=0).round(2),
        "stor_gw_p10": np.percentile(stor_gw, 10, axis=0).round(2),
        "stor_gw_p90": np.percentile(stor_gw, 90, axis=0).round(2),
        "gas_gw_traj": gas_traj.round(2),
        "crossover_p10": lo,
        "crossover_med": med,
        "crossover_p90": hi,
    })
    res.write_csv(TABLES / "q7_bayesian_crossover.csv")
    np.save(TABLES / "q7_crossover_dist.npy", crossover_years)
    print(f"  crossover stockage>=gaz : médiane {med:.0f} (P10 {lo:.0f} – P90 {hi:.0f})")
    print(f"  gaz op 2026 (GEM) : {gas_gw_2026:.0f} GW | stockage cumulé 2026 : "
          f"{float(np.exp(np.median(a_s + b_s * (2026-2000)/10))) / 1e3:.1f} GW GEM-tracked")


def q14_growth(feat: pl.DataFrame) -> pl.DataFrame:
    """Q14 — proxy : croissance capacité brute vs pipeline (proxy brevets)."""
    df = ensure_processed()
    op = df.filter(pl.col("status").is_in(ACTIVE_STATUSES))
    old = (
        op.filter(pl.col("year").is_not_null() & (pl.col("year") <= 2015))
        .group_by("iso3").agg(pl.col("capacity_mw").sum().alias("cap_2015_mw"))
    )
    new = (
        op.filter(pl.col("year").is_not_null() & (pl.col("year") >= 2021))
        .group_by("iso3").agg(pl.col("capacity_mw").sum().alias("cap_2021_mw"))
    )
    out = (
        feat.select(["iso3", "pipeline_ratio"])
        .join(old, on="iso3", how="left").join(new, on="iso3", how="left")
        .fill_null(0.0)
        .with_columns(
            ((pl.col("cap_2021_mw") - pl.col("cap_2015_mw")) / (pl.col("cap_2015_mw") + 1.0))
            .alias("growth_2015_2021"),
        )
        .with_columns(
            (pl.col("pipeline_ratio") / (pl.col("growth_2015_2021") + 1e-6)).alias("ip_vs_cap_proxy"),
        )
        .sort("ip_vs_cap_proxy", descending=True)
    )
    out.write_csv(TABLES / "q14_growth_proxy.csv")
    return out


def q15_resilience(feat: pl.DataFrame) -> pl.DataFrame:
    """Q15 — diversité (entropie de Shannon) + redondance par pays."""
    cols = ["fossil_mw", "nuclear_mw", "hydro_mw", "var_mw", "bio_geo_mw"]
    out = []
    for r in feat.select(["iso3"] + cols).iter_rows(named=True):
        parts = np.array([r[c] for c in cols], dtype=float)
        parts = parts[parts > 1.0]
        if parts.sum() <= 0:
            ent = 0.0
        else:
            ent = stats.entropy(parts / parts.sum())
        out.append({"iso3": r["iso3"], "shannon_entropy": round(ent, 3),
                    "n_tech_gt1gw": int((parts > 1000.0).sum())})
    res = pl.DataFrame(out).sort("shannon_entropy", descending=True)
    res.write_csv(TABLES / "q15_resilience_entropy.csv")
    return res


def main() -> None:
    feat = load_country_features()
    print("=== Q9 — indice de cascade (top 15) ===")
    q9 = q9_cascade(feat)
    print(q9.select(["iso3", "hydro_share", "fossil_share", "var_share", "cascade_index"])
          .head(15).to_pandas().to_string(index=False))
    print("\n=== Q10 — clusters hubris ===")
    q10 = q10_hubris(feat)
    hub = q10.filter(pl.col("is_hubris")).sort("var_share", descending=True)
    print(hub.select(["iso3", "var_share", "storage_share", "pipeline_ratio", "total_mw"])
          .head(12).to_pandas().to_string(index=False))
    print("\n=== Q7 — modèle bayésien pyro ===")
    q7_bayesian_crossover()
    print("\n=== Q14 — proxy croissance ===")
    q14 = q14_growth(feat)
    print(q14.select(["iso3", "growth_2015_2021", "pipeline_ratio", "ip_vs_cap_proxy"])
          .head(10).to_pandas().to_string(index=False))
    print("\n=== Q15 — diversité (entropie) ===")
    q15 = q15_resilience(feat)
    print(q15.head(12).to_pandas().to_string(index=False))
    print(q15.tail(8).to_pandas().to_string(index=False))


if __name__ == "__main__":
    main()
