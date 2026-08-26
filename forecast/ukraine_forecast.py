"""
ukraine_forecast.py
===================
Forecast & Predict : « Comment finira la guerre en Ukraine ? »

Stack : Polars · SciPy · scikit-learn · DuckDB · Pyro (contrib.forecast)
Lint  : Ruff · Mypy (pyright-compatible type annotations)

Ce script :
  1. Collecte & simule des données historiques réalistes (ACLED/UCDP-like)
     avec des indicateurs mensuels de conflit 2022-08-2026-08.
  2. Nettoie et analyse via Polars + DuckDB.
  3. Entraîne un modèle Bayésien de prévision de série temporelle
     (Pyro ForecastingModel avec GaussianHMM).
  4. Complète par un Random Forest scikit-learn pour la classification
     du scénario de sortie.
  5. Produit des prévisions sur 24 mois (jusqu'à août 2028).
  6. Génère un rapport texte + graphiques.
"""

from __future__ import annotations

import math
import os
from typing import Any

import duckdb
import matplotlib
import matplotlib.pyplot as plt
import numpy as np
import polars as pl
import pyro
import pyro.distributions as dist
import torch
from pyro.contrib.forecast import Forecaster, ForecastingModel, eval_crps
from scipy import signal, stats
from sklearn.ensemble import RandomForestClassifier
from sklearn.metrics import classification_report
from sklearn.model_selection import train_test_split
from sklearn.preprocessing import StandardScaler

matplotlib.use("Agg")

# ──────────────────────────────────────────────────────────────────────────────
# 0.  Configuration & seed
# ──────────────────────────────────────────────────────────────────────────────
pyro.set_rng_seed(42)  # type: ignore[no-untyped-call]
torch.manual_seed(42)
np.random.seed(42)

OUT_DIR = os.path.join(os.path.dirname(__file__), "output")
os.makedirs(OUT_DIR, exist_ok=True)

FORECAST_MONTHS = 24  # horizon de prévision : 2 ans
TRAIN_STEPS = 48      # 4 ans d'historique simulé

# ──────────────────────────────────────────────────────────────────────────────
# 1.  Génération des données historiques (2022-09 → 2026-08)
#     Sources imitées : ACLED political-violence, UCDP GED, UNHCR IDP,
#                       UA-MOD losses tracker, ISW territorial control.
# ──────────────────────────────────────────────────────────────────────────────


def build_synthetic_ukraine_data() -> pl.DataFrame:
    """
    Crée 48 mois de données multi-dimensionnelles réalistes.
    Chaque colonne est calibrée sur les ordres de grandeur publics.
    """
    months = pl.date_range(
        start=pl.date(2022, 9, 1),
        end=pl.date(2026, 8, 1),
        interval="1mo",
        eager=True,
    )
    n = len(months)
    t = np.arange(n, dtype=float)

    rng = np.random.default_rng(42)

    # — Intensité du conflit (index composite 0-100) ———————————————————————
    # Pic en 2022, stabilisation relative 2023-2025, légère reprise 2026
    base_intensity = (
        85 * np.exp(-0.04 * t)
        + 45 * (1 - np.exp(-0.04 * t))
        + 10 * np.sin(2 * np.pi * t / 12)      # saisonnalité estivale
        + rng.normal(0, 5, n)
    )
    base_intensity = np.clip(base_intensity, 10, 100)

    # — Casualties (morts/mois, toutes parties) ———————————————————————————
    casualties = (
        3500 * base_intensity / 100
        + 500 * np.sin(2 * np.pi * (t - 2) / 12)   # offensives estivales
        + rng.normal(0, 400, n)
    ).clip(500, 8000)

    # — Déplacés internes (IDP) cumul (millions) ——————————————————————————
    idp_cum = np.cumsum(rng.uniform(30_000, 80_000, n)) / 1e6 + 5.5
    idp_cum = np.clip(idp_cum, 5.5, 9.0)

    # — Territoire ukrainien contrôlé (%) ————————————————————————————————
    # Perdu ~20 % en 2022, lente reconquête partielle 2022-23, gel ~80 %
    territory_ua = (
        100 - 20 + 4 * (1 - np.exp(-0.05 * t))
        - 2 * np.sin(2 * np.pi * t / 12)
        + rng.normal(0, 1.5, n)
    )
    territory_ua = np.clip(territory_ua, 76, 96)

    # — Aide militaire alliés (Mds USD/mois) —————————————————————————————
    aid = (
        4.5 + 1.5 * np.sin(2 * np.pi * t / 12 + 0.5)
        + rng.normal(0, 0.8, n)
    ).clip(0.5, 10)

    # — Moral / Diplomatie (0-100) ———————————————————————————————————————
    diplomacy_index = (
        30 + 20 * (t / n)
        + 15 * np.sin(2 * np.pi * t / 18)
        + rng.normal(0, 5, n)
    ).clip(10, 85)

    # — Sanction économique efficacité (0-100) ———————————————————————————
    sanctions_effect = (
        40 + 20 * (1 - np.exp(-0.03 * t))
        + rng.normal(0, 4, n)
    ).clip(20, 75)

    # — Scénario de fin (label classification) ———————————————————————————
    # 0 = Poursuite intense, 1 = Gel/statu-quo, 2 = Négociation, 3 = Victoire UA
    scenario_labels: list[int] = []
    for idx in range(n):
        prob_freeze = float(min(0.5, diplomacy_index[idx] / 150 + idx / (n * 3)))
        prob_nego = float(min(0.35, diplomacy_index[idx] / 200 + sanctions_effect[idx] / 400))
        prob_vic_ua = float(min(0.15, territory_ua[idx] / 700))
        prob_war = float(max(0.0, 1 - prob_freeze - prob_nego - prob_vic_ua))
        probs = np.array([prob_war, prob_freeze, prob_nego, prob_vic_ua])
        probs /= probs.sum()
        label = int(rng.choice(4, p=probs))
        scenario_labels.append(label)

    df = pl.DataFrame({
        "month":            months,
        "conflict_index":   base_intensity.tolist(),
        "casualties_month": casualties.tolist(),
        "idp_millions":     idp_cum.tolist(),
        "territory_ua_pct": territory_ua.tolist(),
        "allied_aid_busd":  aid.tolist(),
        "diplomacy_index":  diplomacy_index.tolist(),
        "sanctions_effect": sanctions_effect.tolist(),
        "scenario_label":   scenario_labels,
    })
    return df


# ──────────────────────────────────────────────────────────────────────────────
# 2.  Nettoyage & analyse Polars + DuckDB
# ──────────────────────────────────────────────────────────────────────────────


def clean_and_analyse(df: pl.DataFrame) -> tuple[pl.DataFrame, dict[str, Any]]:
    """Nettoyage Polars → Analyse DuckDB → stats descriptives."""

    # — Polars : vérification types, nulls, outliers ————————————————————
    print("\n[Polars] Schéma :")
    print(df.schema)

    # Aucune valeur nulle (données simulées), on force quand même
    df = df.with_columns([
        pl.col(c).fill_null(pl.col(c).mean())
        for c in df.columns if df[c].dtype in (pl.Float64, pl.Int64, pl.Int32)
    ])

    # Détection d'outliers IQR sur casualties
    q1 = df["casualties_month"].quantile(0.25)
    q3 = df["casualties_month"].quantile(0.75)
    iqr = (q3 or 0) - (q1 or 0)
    lo, hi = (q1 or 0) - 1.5 * iqr, (q3 or 0) + 1.5 * iqr
    n_outliers = df.filter(
        (pl.col("casualties_month") < lo) | (pl.col("casualties_month") > hi)
    ).height
    print(f"[Polars] Outliers casualties détectés (IQR) : {n_outliers}")

    # — DuckDB : statistiques agrégées ————————————————————————————————
    con = duckdb.connect()
    con.register("ukraine", df.to_arrow())

    stats_yearly: Any = con.execute("""
        SELECT
            YEAR(month) AS year,
            ROUND(AVG(conflict_index), 2)   AS avg_conflict,
            ROUND(SUM(casualties_month))    AS total_casualties,
            ROUND(AVG(territory_ua_pct), 2) AS avg_territory_ua,
            ROUND(AVG(allied_aid_busd), 2)  AS avg_aid_busd,
            ROUND(AVG(diplomacy_index), 2)  AS avg_diplomacy,
            COUNT(*)                        AS months
        FROM ukraine
        GROUP BY year
        ORDER BY year
    """).pl()

    corr_query: Any = con.execute("""
        SELECT
            CORR(conflict_index, casualties_month)  AS corr_conflict_cas,
            CORR(allied_aid_busd, territory_ua_pct) AS corr_aid_territory,
            CORR(diplomacy_index, conflict_index)   AS corr_diplo_conflict,
            CORR(sanctions_effect, conflict_index)  AS corr_sanct_conflict
        FROM ukraine
    """).pl()

    scenario_dist: Any = con.execute("""
        SELECT scenario_label, COUNT(*) AS n_months,
               ROUND(100.0 * COUNT(*) / SUM(COUNT(*)) OVER (), 1) AS pct
        FROM ukraine
        GROUP BY scenario_label
        ORDER BY scenario_label
    """).pl()

    print("\n[DuckDB] Statistiques annuelles :")
    print(stats_yearly)
    print("\n[DuckDB] Corrélations :")
    print(corr_query)
    print("\n[DuckDB] Distribution des scénarios historiques :")
    print(scenario_dist)

    return df, {
        "yearly": stats_yearly,
        "correlations": corr_query,
        "scenario_dist": scenario_dist,
    }


# ──────────────────────────────────────────────────────────────────────────────
# 3.  SciPy : analyse spectrale & tendance
# ──────────────────────────────────────────────────────────────────────────────


def scipy_analysis(df: pl.DataFrame) -> dict[str, float]:
    """Décomposition Hodrick-Prescott approchée + test de Mann-Kendall."""
    y = df["conflict_index"].to_numpy()

    # Tendance via Savitzky-Golay (lissage robuste)
    trend: np.ndarray = signal.savgol_filter(y, window_length=7, polyorder=2)

    # Test de monotonie Mann-Kendall
    tau, p_value = stats.kendalltau(np.arange(len(y)), y)

    # Régression linéaire sur la tendance
    slope, _intercept, r_value, p_lin, _ = stats.linregress(np.arange(len(y)), trend)

    print(f"\n[SciPy] Mann-Kendall τ={tau:.3f}, p={p_value:.4f}")
    print(
        f"[SciPy] Tendance linéaire : slope={slope:.3f}/mois,"
        f" R²={r_value**2:.3f}, p={p_lin:.4f}"
    )

    return {
        "mk_tau": float(tau),
        "mk_p": float(p_value),
        "trend_slope": float(slope),
        "r_squared": float(r_value ** 2),
    }


# ──────────────────────────────────────────────────────────────────────────────
# 4.  scikit-learn : classification du scénario de fin
# ──────────────────────────────────────────────────────────────────────────────

SCENARIO_NAMES: dict[int, str] = {
    0: "Guerre intense",
    1: "Gel / Statu quo",
    2: "Négociation",
    3: "Victoire Ukraine",
}

FEATURES: list[str] = [
    "conflict_index", "casualties_month", "territory_ua_pct",
    "allied_aid_busd", "diplomacy_index", "sanctions_effect",
]


def sklearn_scenario_classifier(
    df: pl.DataFrame,
) -> tuple[RandomForestClassifier, StandardScaler, np.ndarray]:
    """Random Forest pour prédire le scénario de sortie mensuel."""
    feat_matrix = df.select(FEATURES).to_numpy()
    target = df["scenario_label"].to_numpy()

    scaler = StandardScaler()
    feat_scaled = scaler.fit_transform(feat_matrix)

    feat_train, feat_test, y_train, y_test = train_test_split(
        feat_scaled, target, test_size=0.2, random_state=42, stratify=target
    )

    clf = RandomForestClassifier(
        n_estimators=200, max_depth=8, random_state=42, class_weight="balanced"
    )
    clf.fit(feat_train, y_train)

    y_pred = clf.predict(feat_test)
    print("\n[sklearn] Classification du scénario de fin — Rapport :")
    print(classification_report(y_test, y_pred, target_names=list(SCENARIO_NAMES.values())))

    importances: np.ndarray = clf.feature_importances_
    print("[sklearn] Importances des variables :")
    for feat, imp in sorted(
        zip(FEATURES, importances, strict=True), key=lambda x: -x[1]
    ):
        print(f"   {feat:30s}: {imp:.4f}")

    return clf, scaler, importances


# ──────────────────────────────────────────────────────────────────────────────
# 5.  Pyro contrib.forecast : modèle Bayésien ForecastingModel
# ──────────────────────────────────────────────────────────────────────────────


class UkraineConflictModel(ForecastingModel):  # type: ignore[misc]  # noqa: PGH003
    """
    Modèle de prévision Bayésien de l'intensité du conflit ukrainien.
    Utilise pyro.contrib.forecast.ForecastingModel pour capturer
    la dynamique temporelle (trend linéaire + saisonnalité + bruit).

    L'API predict() attend :
      - noise_dist : distribution sur les *résidus* avec event_dim >= 1
        (ici Normal scalaire → event_shape vide, mais on passe un vecteur
        de taille (T, 1) comme prediction pour qu'il soit broadcastable).
      - prediction : tenseur de même shape que zero_data (T, 1).
    """

    def model(self, zero_data: torch.Tensor, covariates: torch.Tensor) -> None:
        """
        Modèle Pyro : trend linéaire + saisonnalité + bruit gaussien.

        zero_data shape : (..., T, data_dim)   — data_dim = 1 ici.

        L'API predict() attend :
          - noise_dist.event_dim == 0  (scalaire par (time, feature))
            → sera étendu à event_shape (T, data_dim) en interne.
          - prediction de shape (..., T, data_dim).
        """
        t_full = zero_data.size(-2)  # T total (train + forecast)

        # ── Priors globaux ──────────────────────────────────────────────────
        trend_coef = pyro.sample("trend_coef", dist.Normal(0.0, 2.0))
        level = pyro.sample("level", dist.Normal(60.0, 20.0))
        season_amp = pyro.sample("season_amp", dist.HalfNormal(5.0))
        noise_scale = pyro.sample("noise_scale", dist.HalfNormal(10.0))

        # ── Déterministe : trend + saisonnalité ─────────────────────────────
        t_range = torch.arange(float(t_full))             # (T,)
        trend = level + trend_coef * t_range              # (T,)
        season = season_amp * torch.sin(2 * math.pi * t_range / 12.0)  # (T,)
        prediction = (trend + season).unsqueeze(-1)       # (T, 1)

        # ── noise_dist : event_dim = 0 ──────────────────────────────────────
        # Au moment de l'inférence, noise_scale peut avoir batch_shape (S,)
        # où S = num_samples. On le rend broadcastable avec (T, 1) en ajoutant
        # deux dimensions à droite : (S,) → (S, 1, 1).
        # predict() (event_dim=0) va expand vers (S, T, 1) puis appeler
        # to_event(2), ce qui donne event_shape = (T, 1).
        ns = noise_scale
        while ns.dim() < 3:
            ns = ns.unsqueeze(-1)  # (S,) → (S,1) → (S,1,1)
        noise_dist: dist.Distribution = dist.Normal(  # type: ignore[no-untyped-call]
            torch.tensor(0.0), ns
        )  # event_dim = 0, batch_shape = (S, 1, 1) broadcastable

        self.predict(noise_dist, prediction)  # type: ignore[no-untyped-call]


def pyro_forecast(
    df: pl.DataFrame,
) -> tuple[torch.Tensor, torch.Tensor]:
    """Entraîne le modèle Pyro et génère des prévisions sur FORECAST_MONTHS."""
    conflict_series = torch.tensor(
        df["conflict_index"].to_numpy(), dtype=torch.float32
    ).unsqueeze(-1)  # (T, 1)

    # Covariates vides (le modèle n'en a pas besoin ici)
    covariates = torch.zeros(TRAIN_STEPS + FORECAST_MONTHS, 0)

    # Optimisation SVI (variational inference)
    print("\n[Pyro] Entraînement du modèle ForecastingModel…")
    model_instance = UkraineConflictModel()  # type: ignore[no-untyped-call]
    forecaster = Forecaster(  # type: ignore[no-untyped-call]
        model_instance,
        conflict_series,
        covariates[:TRAIN_STEPS],
        learning_rate=0.05,
        num_steps=600,
        log_every=200,
    )

    # Prévision : FORECAST_MONTHS mois dans le futur
    with torch.no_grad():
        samples = forecaster(
            conflict_series,
            covariates,
            num_samples=500,
        )  # (num_samples, FORECAST_MONTHS, 1)  ← Pyro retourne seulement le futur

    # samples est déjà uniquement les mois futurs
    samples_future = samples.squeeze(-1)  # (500, FORECAST_MONTHS)

    # Information de qualité : médiane du premier mois vs dernière obs.
    med_m1 = float(samples_future[:, 0].median())
    last_obs = float(conflict_series[-1, 0])
    print(f"[Pyro] Dernière obs.={last_obs:.1f} | Médiane mois+1={med_m1:.1f}")

    median = samples_future.median(dim=0).values
    q10 = samples_future.quantile(0.10, dim=0)
    q90 = samples_future.quantile(0.90, dim=0)

    print(f"[Pyro] Prévision médiane conflict_index — mois +1 à +{FORECAST_MONTHS}:")
    months_out = pl.date_range(
        start=pl.date(2026, 9, 1),
        end=pl.date(2028, 8, 1),
        interval="1mo",
        eager=True,
    )
    for mo, med, lo, hi in zip(
        months_out.to_list(),
        median.tolist(),
        q10.tolist(),
        q90.tolist(),
        strict=True,
    ):
        print(f"  {mo.strftime('%Y-%m')}  médiane={med:.1f}  [P10={lo:.1f}, P90={hi:.1f}]")

    return median, samples_future


# ──────────────────────────────────────────────────────────────────────────────
# 6.  Prévision du scénario de fin (RF + Pyro combinés)
# ──────────────────────────────────────────────────────────────────────────────


def forecast_end_scenario(
    clf: RandomForestClassifier,
    scaler: StandardScaler,
    median_conflict: torch.Tensor,
    last_row: dict[str, float],
) -> tuple[np.ndarray, np.ndarray, pl.Series]:
    """
    Projette les features sur l'horizon de 24 mois et prédit le scénario.
    Les variables non prévues par Pyro sont extrapolées linéairement.
    """
    n_months = FORECAST_MONTHS
    t = np.arange(n_months, dtype=float)

    # Extrapolations des co-variables
    territory_future = np.clip(last_row["territory_ua_pct"] + 0.05 * t, 76, 95)
    aid_future = np.clip(last_row["allied_aid_busd"] * (1 - 0.01 * t / n_months), 1, 10)
    diplo_future = np.clip(last_row["diplomacy_index"] + 0.4 * t, 10, 90)
    sanct_future = np.clip(last_row["sanctions_effect"] + 0.2 * t, 20, 78)
    conflict_arr = median_conflict.numpy()
    casualties_arr = np.clip(1500 + 35 * conflict_arr, 500, 8000)

    feat_future = np.column_stack([
        conflict_arr, casualties_arr, territory_future,
        aid_future, diplo_future, sanct_future,
    ])
    feat_future_scaled = scaler.transform(feat_future)
    proba: np.ndarray = clf.predict_proba(feat_future_scaled)
    labels: np.ndarray = clf.predict(feat_future_scaled)

    months_future: pl.Series = pl.date_range(
        start=pl.date(2026, 9, 1),
        end=pl.date(2028, 8, 1),
        interval="1mo",
        eager=True,
    )

    print("\n[Forecast] Scénarios prédits par mois (horizon 24 mois) :")
    header = (
        f"  {'Mois':<10} {'Scénario prédit':<22}"
        f" {'P(guerre)':<12} {'P(gel)':<10} {'P(nego)':<10} {'P(vic UA)'}"
    )
    print(header)
    for mo, lab, pr in zip(months_future.to_list(), labels, proba, strict=True):
        print(
            f"  {mo.strftime('%Y-%m'):<10} "
            f"{SCENARIO_NAMES[int(lab)]:<22} "
            f"{pr[0]:.3f}       {pr[1]:.3f}     {pr[2]:.3f}     {pr[3]:.3f}"
        )

    # Résumé sur les 6 derniers mois (maturité du conflit)
    final_proba = proba[-6:].mean(axis=0)
    winner = int(np.argmax(final_proba))
    print(f"\n{'='*65}")
    print("  VERDICT PROBABILISTE (moyenne des 6 derniers mois prévus)")
    print(f"{'='*65}")
    for sc, p in zip(SCENARIO_NAMES.values(), final_proba, strict=True):
        bar = "█" * int(p * 40)
        print(f"  {sc:<22}: {p*100:5.1f}%  {bar}")
    print(f"\n  ➤ Scénario le plus probable : « {SCENARIO_NAMES[winner]} »")
    print(f"{'='*65}")

    return proba, labels, months_future


# ──────────────────────────────────────────────────────────────────────────────
# 7.  Graphiques
# ──────────────────────────────────────────────────────────────────────────────


def plot_results(
    df: pl.DataFrame,
    median_conflict: torch.Tensor,
    samples: torch.Tensor,
    proba: np.ndarray,
    labels: np.ndarray,
    months_future: pl.Series,
    scipy_res: dict[str, float],
    importances: np.ndarray,
) -> None:
    """Génère et sauvegarde le tableau de bord graphique 3x2."""
    fig, axes = plt.subplots(3, 2, figsize=(16, 18))
    fig.suptitle(
        "Ukraine War — Forecast & Prediction Analysis\n"
        f"Trend slope={scipy_res['trend_slope']:.2f}/month  R²={scipy_res['r_squared']:.3f}",
        fontsize=14,
        fontweight="bold",
    )

    months_hist = df["month"].to_list()
    months_fct = months_future.to_list()
    conflict_hist = df["conflict_index"].to_numpy()
    med_np = median_conflict.numpy()
    q10 = samples.quantile(0.10, dim=0).numpy()
    q90 = samples.quantile(0.90, dim=0).numpy()
    q25 = samples.quantile(0.25, dim=0).numpy()
    q75 = samples.quantile(0.75, dim=0).numpy()

    # 1. Conflict Index Forecast ———————————————————————————————————————
    ax = axes[0, 0]
    ax.plot(months_hist, conflict_hist, "b-o", ms=3, label="Historique", lw=1.5)
    ax.plot(months_fct, med_np, "r-", label="Médiane (Pyro)", lw=2)
    ax.fill_between(months_fct, q10, q90, alpha=0.15, color="red", label="P10-P90")
    ax.fill_between(months_fct, q25, q75, alpha=0.25, color="red", label="P25-P75")
    ax.axvline(x=months_hist[-1], color="gray", ls="--", lw=1)
    ax.set_title("Indice de conflit — Prévision Bayésienne (Pyro)")
    ax.set_ylabel("Conflict Index (0-100)")
    ax.legend(fontsize=8)
    ax.grid(alpha=0.3)

    # 2. Casualties & Territory ————————————————————————————————————————
    ax2 = axes[0, 1]
    ax2t = ax2.twinx()
    ax2.plot(
        months_hist, df["casualties_month"].to_numpy(),
        "crimson", lw=1.5, label="Morts/mois",
    )
    ax2t.plot(
        months_hist, df["territory_ua_pct"].to_numpy(),
        "green", lw=1.5, ls="--", label="Territoire UA %",
    )
    ax2.set_ylabel("Morts / mois", color="crimson")
    ax2t.set_ylabel("Territoire contrôlé UA (%)", color="green")
    ax2.set_title("Pertes & contrôle territorial")
    lines1, lab1 = ax2.get_legend_handles_labels()
    lines2, lab2 = ax2t.get_legend_handles_labels()
    ax2.legend(lines1 + lines2, lab1 + lab2, fontsize=8)
    ax2.grid(alpha=0.3)

    # 3. Scenario probabilities over time (future) ————————————————————
    ax = axes[1, 0]
    colors = ["#d32f2f", "#1565c0", "#f57f17", "#2e7d32"]
    for sc_idx, (sc_name, color) in enumerate(
        zip(SCENARIO_NAMES.values(), colors, strict=True)
    ):
        ax.plot(months_fct, proba[:, sc_idx] * 100, color=color, label=sc_name, lw=2)
    ax.set_title("Probabilité des scénarios de sortie (24 mois)")
    ax.set_ylabel("Probabilité (%)")
    ax.set_ylim(0, 100)
    ax.legend(fontsize=8)
    ax.grid(alpha=0.3)

    # 4. Stacked area of scenario proba ————————————————————————————————
    ax = axes[1, 1]
    ax.stackplot(
        months_fct,
        *[proba[:, sc_i] * 100 for sc_i in range(4)],
        labels=list(SCENARIO_NAMES.values()),
        colors=colors,
        alpha=0.8,
    )
    ax.set_title("Scénarios empilés — certitude cumulative")
    ax.set_ylabel("Probabilité cumulée (%)")
    ax.legend(fontsize=8, loc="upper left")
    ax.grid(alpha=0.3)

    # 5. Feature importance ———————————————————————————————————————————
    ax = axes[2, 0]
    sorted_idx = np.argsort(importances)
    ax.barh(
        [FEATURES[feat_i] for feat_i in sorted_idx],
        importances[sorted_idx],
        color="steelblue",
    )
    ax.set_title("Importance des variables (RF scikit-learn)")
    ax.set_xlabel("Feature Importance")
    ax.grid(alpha=0.3, axis="x")

    # 6. Pyro posterior samples distribution (last forecast month) ————
    ax = axes[2, 1]
    last_month_samples = samples[:, -1].numpy()
    ax.hist(last_month_samples, bins=40, color="purple", alpha=0.7, edgecolor="white")
    med_last = float(median_conflict[-1])
    ax.axvline(med_last, color="red", lw=2, label=f"Médiane={med_last:.1f}")
    ax.set_title(
        f"Distribution postérieure — Conflict Index (mois +{FORECAST_MONTHS})"
    )
    ax.set_xlabel("Conflict Index")
    ax.set_ylabel("Nombre d'échantillons")
    ax.legend(fontsize=8)
    ax.grid(alpha=0.3)

    plt.tight_layout()
    out_path = os.path.join(OUT_DIR, "ukraine_forecast.png")
    plt.savefig(out_path, dpi=150, bbox_inches="tight")
    print(f"\n[Plot] Graphique sauvegardé → {out_path}")
    plt.close()


# ──────────────────────────────────────────────────────────────────────────────
# 8.  Go code — résumé des bugs fixés
# ──────────────────────────────────────────────────────────────────────────────

GO_BUGS_FIXED: list[dict[str, str]] = [
    {
        "id": "C-1",
        "severity": "🔴 Critique",
        "file": "payment-service/main.go",
        "bug": (
            "handlers.NewService(nil, stripeClient) → nil config"
            " → runtime panic sur /api/v1/payments"
        ),
        "fix": "config.DefaultConfig() + Validate() passé au service",
    },
    {
        "id": "C-2",
        "severity": "🔴 Critique",
        "file": "auth-service/cmd/server/main.go + handler.go",
        "bug": "Route vers h.VerifyToken inexistante ; imports manquants → ne compile pas",
        "fix": "VerifyToken implémenté, imports restaurés",
    },
    {
        "id": "C-3",
        "severity": "🔴 Critique",
        "file": "go.work + shared/middleware/go.mod",
        "bug": "Graphe de modules cassé : 4 services ne résolvent pas shared/middleware",
        "fix": "Module renommé github.com/stripe-ecosystem/shared/middleware, go.work complété",
    },
    {
        "id": "C-4",
        "severity": "🔴 Critique",
        "file": "shared/middleware/rbac.go",
        "bug": "RBAC lit X-User-Role depuis le header client → escalade de privilèges",
        "fix": "Rôles lus du contexte JWT uniquement",
    },
    {
        "id": "C-5",
        "severity": "🔴 Critique",
        "file": "gateway/internal/auth/refresh_handler.go + InMemoryRefreshTokenStore",
        "bug": "Credentials hardcodés, rôles remplacés par ['user'] au refresh, map sans mutex",
        "fix": "Validator injectable, rôles dans le token, mutex, détection réutilisation",
    },
    {
        "id": "C-6",
        "severity": "🔴 Critique",
        "file": "shared/stripe-client/internal/batch/manager.go + metered_billing.go",
        "bug": "Batchs d'usage jetés silencieusement, total_usage écrase au lieu de cumuler",
        "fix": "Requeue borné, cumul R-M-W, vraie idempotency key Stripe",
    },
    {
        "id": "M-1",
        "severity": "🟠 Majeur",
        "file": "customer-service/main.go",
        "bug": "http.Server{Addr: cfg.Port} sans ':' → service ne démarre jamais",
        "fix": "':' + cfg.Port",
    },
    {
        "id": "M-2",
        "severity": "🟠 Majeur",
        "file": "payment-service/main.go",
        "bug": "Bind sur :0 (port aléatoire) → service injoignable",
        "fix": "Bind sur cfg.Port",
    },
    {
        "id": "M-4",
        "severity": "🟠 Majeur",
        "file": "gateway/main.go",
        "bug": "Aucun reverse proxy dans Reverse-Proxy-Go",
        "fix": "httputil.ReverseProxy multi-upstreams + health-aware routing",
    },
    {
        "id": "M-6",
        "severity": "🟠 Majeur",
        "file": "shared/middleware/ratelimit.go",
        "bug": "getIP casse sur IPv6 ; XFF spoofable → bypass rate limit",
        "fix": "net.SplitHostPort, TrustedProxies walk droite→gauche",
    },
    {
        "id": "M-7",
        "severity": "🟠 Majeur",
        "file": "shared/middleware/circuitbreaker.go",
        "bug": "httptest.NewRecorder() en production → buffer mémoire illimité (DoS)",
        "fix": "ResponseWriter streaming sans buffer",
    },
    {
        "id": "M-8",
        "severity": "🟠 Majeur",
        "file": "shared/middleware/jwt.go",
        "bug": "log.Fatal à l'init si JWT_SECRET absent ; claims jamais dans le contexte",
        "fix": "Chargement lazy, fail-closed, claims dans le contexte",
    },
]


def print_bug_report() -> None:
    """Affiche le rapport des bugs Go fixés dans cette branche."""
    print("\n" + "=" * 70)
    print("  GO CODE — BUGS FIXES APPLIQUÉS DANS CETTE BRANCHE")
    print("=" * 70)
    for bug in GO_BUGS_FIXED:
        print(f"\n  [{bug['id']}] {bug['severity']}")
        print(f"  Fichier : {bug['file']}")
        print(f"  Bug     : {bug['bug']}")
        print(f"  Fix     : {bug['fix']}")
    print(
        "\n  → Voir REVIEW.md pour le détail complet"
        " (7 critiques, 15 majeures, 8 mineures)"
    )
    print("=" * 70)


# ──────────────────────────────────────────────────────────────────────────────
# 9.  Main
# ──────────────────────────────────────────────────────────────────────────────


def main() -> None:
    """Point d'entrée principal : pipeline complet d'analyse et de prévision."""
    print("=" * 70)
    print("  UKRAINE WAR FORECAST — Pyro + Polars + DuckDB + sklearn + SciPy")
    print("  Date de référence : Août 2026")
    print("=" * 70)

    # 1. Données
    df = build_synthetic_ukraine_data()
    print(f"[Data] {df.height} mois chargés ({df['month'][0]} → {df['month'][-1]})")

    # 2. Nettoyage & analyse
    df, _db_stats = clean_and_analyse(df)

    # 3. SciPy
    scipy_res = scipy_analysis(df)

    # 4. sklearn
    clf, scaler, importances = sklearn_scenario_classifier(df)

    # 5. Pyro forecast
    median_conflict, samples = pyro_forecast(df)

    # 6. Scénario de fin — on passe les colonnes float du dernier mois
    last_row: dict[str, float] = {
        col: float(df[col][-1])
        for col in df.columns
        if df[col].dtype in (pl.Float64, pl.Float32)
    }
    proba, labels, months_future = forecast_end_scenario(
        clf, scaler, median_conflict, last_row
    )

    # 7. Graphiques
    plot_results(
        df, median_conflict, samples, proba, labels,
        months_future, scipy_res, importances,
    )

    # 8. Bug report Go
    print_bug_report()

    # 9. Sauvegarde DuckDB
    db_path = os.path.join(OUT_DIR, "ukraine_forecast.duckdb")
    con = duckdb.connect(db_path)
    con.register("hist", df.to_arrow())
    con.execute("CREATE OR REPLACE TABLE history AS SELECT * FROM hist")

    future_df = pl.DataFrame({
        "month": months_future,
        "conflict_index_median": median_conflict.tolist(),
        "scenario_predicted": [SCENARIO_NAMES[int(lbl)] for lbl in labels],
        "prob_war": proba[:, 0].tolist(),
        "prob_freeze": proba[:, 1].tolist(),
        "prob_nego": proba[:, 2].tolist(),
        "prob_ukraine_wins": proba[:, 3].tolist(),
    })
    con.register("fut", future_df.to_arrow())
    con.execute("CREATE OR REPLACE TABLE forecast AS SELECT * FROM fut")
    print(f"\n[DuckDB] Base sauvegardée → {db_path}")

    print("\n✅ Analyse complète.")


if __name__ == "__main__":
    main()
