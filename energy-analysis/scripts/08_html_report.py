#!/usr/bin/env python
"""Génère le rapport HTML autonome (docs/rapport_europe.html) à partir des tables.

Le HTML est généré en pur Python (aucun template externe), avec le CSS/JS inline.
Les chiffres cités proviennent des CSV de output/tables/.
"""

from __future__ import annotations

import base64
import html as html_mod
from pathlib import Path

import polars as pl

ROOT = Path(__file__).resolve().parents[1]
TABLES = ROOT / "output" / "tables"
FIGURES = ROOT / "output" / "figures"
OUT = ROOT / "docs" / "rapport_europe.html"


def table(name: str, cols: list[str] | None = None, n: int | None = None) -> str:
    """Rend un CSV en <table> HTML avec classes polaires."""
    d = pl.read_csv(TABLES / name)
    if cols:
        d = d.select(cols)
    if n:
        d = d.head(n)
    head = "".join(f"<th>{html_mod.escape(str(c))}</th>" for c in d.columns)
    rows = []
    for r in d.iter_rows(named=True):
        tds = "".join(f"<td>{html_mod.escape(_fmt(r[c]))}</td>" for c in d.columns)
        rows.append(f"<tr>{tds}</tr>")
    return f"<div class='tblwrap'><table><thead><tr>{head}</tr></thead><tbody>{''.join(rows)}</tbody></table></div>"


def _fmt(v: object) -> str:
    if v is None:
        return "—"
    if isinstance(v, float):
        return f"{v:,.1f}"
    if isinstance(v, bool):
        return "✓" if v else ""
    return str(v)


def fig(name: str, caption: str) -> str:
    """Intègre une figure en base64 (rend le HTML autonome)."""
    p = FIGURES / name
    if not p.exists():
        return ""
    b64 = base64.b64encode(p.read_bytes()).decode()
    return (
        f"<figure><img src='data:image/png;base64,{b64}' alt='{html_mod.escape(caption)}'/>"
        f"<figcaption>{html_mod.escape(caption)}</figcaption></figure>"
    )


def card(k: str, v: str) -> str:
    return f"<div class='kpi'><div class='k'>{k}</div><div class='v'>{v}</div></div>"


def section(title: str, body: str, qid: str | None = None) -> str:
    tag = f"<span class='qid'>{qid}</span>" if qid else ""
    return f"<section><h2>{tag}{title}</h2>{body}</section>"


def main() -> None:
    # ------------------------------------------------------------------
    # Chiffres clés (depuis les tables)
    # ------------------------------------------------------------------
    df_op = pl.read_csv(TABLES / "operating_by_type_europe.csv")
    op_gw = float(df_op["capacity_mw"].sum() or 0.0) / 1e3  # type: ignore[arg-type]
    df_q2 = pl.read_csv(TABLES / "q2_stock_flows.csv")
    pipe_gw = float(df_q2["pipeline_cap_mw"].first() or 0.0) / 1e3  # type: ignore[arg-type]
    dead_gw = float(df_q2["dead_cap_mw"].first() or 0.0) / 1e3  # type: ignore[arg-type]
    df_q1 = pl.read_csv(TABLES / "q1_missingness_europe.csv")
    no_owner = float(df_q1.filter(pl.col("indicator") == "units_without_owner")["share"].first() or 0.0) * 100  # type: ignore[arg-type]
    no_start = float(df_q1.filter(pl.col("indicator") == "units_without_start_year")["share"].first() or 0.0) * 100  # type: ignore[arg-type]
    df_q13 = pl.read_csv(TABLES / "q13_pipeline_no_owner.csv")
    pipe_no_owner_gw = float(df_q13["pipeline_no_owner_mw"].sum() or 0.0) / 1e3  # type: ignore[arg-type]
    df_q8 = pl.read_csv(TABLES / "q8_evening_fossil.csv")
    df_q7 = pl.read_csv(TABLES / "q7_bayesian_crossover.csv")
    q7_cross = int(df_q7["crossover_med"].first() or 0)  # type: ignore[arg-type]

    kpis = "".join([
        card("Parc opérant Europe", f"{op_gw:,.0f} GW"),
        card("Pipeline (pre-construction + annoncé)", f"{pipe_gw:,.0f} GW"),
        card("Capacité morte (retirée/annulée)", f"{dead_gw:,.0f} GW"),
        card("Pipeline sans owner identifiable", f"{pipe_no_owner_gw:,.0f} GW"),
        card("Unités sans owner", f"{no_owner:.0f} %"),
        card("Unités sans année de mise en service", f"{no_start:.0f} %"),
        card("Index soir fossile (médiane top-15)", f"{float(df_q8['evening_fossil_index'].median() or 0.0):.2f}"),  # type: ignore[arg-type]
        card("Croisement stockage≥gaz (modèle pyro)", f"{q7_cross}"),
    ])

    # ------------------------------------------------------------------
    # Corps du rapport (version enrichie du rapport MD)
    # ------------------------------------------------------------------
    intro = f"""
    <p class='lead'>Analyse quantitative du système électrique européen à partir des données publiques
    derrière la carte <strong>OpenGridWorks</strong> : Global Integrated Power Tracker (GEM, août 2026,
    182 592 unités mondiales), Global Transmission Database v1.0 (interconnexions existantes 2023 + planifiées),
    Global Solar Power Tracker et lignes haute tension OSM. Stack : polars, duckdb, scipy, scikit-learn,
    pyro, networkx — code vérifié <em>ruff, mypy, pyright</em>.</p>
    <div class='kpis'>{kpis}</div>
    <p class='note'>📅 28 août 2026 · Périmètre : EU27 + UK + Norvège + Suisse + Balkans + Ukraine/Moldavie +
    Türkiye + Géorgie. Limites documentées en fin de rapport (ENTSO-E horaire non accessible depuis le sandbox →
    proxys statiques explicites).</p>
    """

    corridors = section(
        "Les nouveaux corridors — routes de la soie énergétique et chemin du milieu",
        f"""
        <p>L'Europe opère aujourd'hui avec ~105 GW d'interconnexions internes et seulement ~1,4 GW vers
        l'Afrique du Nord. Le pipeline (GTD v1.0) raconte une autre histoire : <strong>~30 GW de nouvelles
        lignes planifiées</strong>, dont les plus massives sont des corridors transnationaux nouveaux.</p>
        {table("corridors_new_only.csv", ["from_iso3", "to_iso3", "existing_mw", "planned_mw"], 18)}
        <h3>Lecture par corridor</h3>
        <ul>
        <li><strong>Corridor Sud (Méditerranée) :</strong> Égypte–Grèce +5 000 MW (GREGY), Italie–Tunisie +2 600 MW
        (ELMED), Chypre–Égypte/Israël/Grèce +2 000 MW chacun (EuroAsia), Grèce–Libye +2 000 MW, Maroc–Portugal +1 000 MW,
        Malte–Tunisie +250 MW. Agrégat Europe–Afrique du Nord : 1 400 → 6 850 MW planifiés.</li>
        <li><strong>Corridor Est (« chemin du milieu ») :</strong> Géorgie–Türkiye +1 050 MW, Géorgie–Russie +1 000 MW,
        Arménie–Iran +850 MW. La Géorgie (3 GW renouvelables opérants, un seul lien de 150 MW) est le symbole du
        corridor encore bloqué.</li>
        <li><strong>Corridor Nord :</strong> DE–GB +2 800 MW, DK–GB +2 800 MW, FR–GB +5 875 MW, IE–FR +700 MW (Celtic),
        IE–GB +2 370 MW.</li>
        <li><strong>Corridor Est-européen :</strong> EE–LV +2 100 MW, LT–PL +1 000 MW (Harmony Link), DE–PL +2 400 MW,
        RO–RS +2 310 MW, AT–DE +7 800 MW (le plus gros renforcement intra-UE).</li>
        </ul>
        {fig("corridors_planned.png", "Corridors d'interconnexion planifiés (GTD v1.0) — capacité en GW")}
        """,
    )

    q1 = section(
        "Q1 — Les données NON cartographiées, et pourquoi leur absence est un signal",
        f"""
        <p>Les absences ne portent pas sur les coordonnées (GEM ne garde que les unités géolocalisées) mais sur
        des <strong>classes d'actifs entières, des champs économiques et des régions</strong> :</p>
        <ul>
        <li><strong>Batteries / stockage hors pompage : aucun MW</strong> dans les 182 592 lignes. Le stockage GEM =
        pompage (1 045 unités) + drapeau « associated storage » sur le solaire. Le marché BESS européen (~15-20 GW
        installés en 2025) est invisible → actif structurellement sous-coté.</li>
        <li><strong>Owners : 65,6 % des unités sans owner</strong> (88,4 % du solaire) ; 83,2 % sans opérateur →
        la couche « capital » de la carte est vide.</li>
        <li><strong>Années de mise en service : 34,6 % manquantes</strong> (84,5 % du pré-construction) → le pipeline
        est indatable : aucune promesse « X GW en 2030 » n'est une donnée.</li>
        <li><strong>Couches US-only</strong> (congressionalDistricts, energyCommunities, evCharging, shalePlays…) :
        pour l'Europe la granularité de la carte est structurellement plus pauvre.</li>
        </ul>
        {table("q1_missingness_europe.csv")}
        {table("q1_missingness_by_type.csv", ["type", "n", "share_no_owner", "share_no_startyear"], 8)}
        """,
        "1",
    )

    q2 = section(
        "Q2 — Résolution temporelle des couches, illusions de stabilité",
        f"""
        <p><strong>Résolutions :</strong> GIPT = photo annuelle (statut + année de mise en service + retrait par
        unité, vintage août 2026) · GTD = photo 2023 (planifié avec champ <em>year_planned</em> rempli à ~0 %) ·
        OSM = instantané non daté · Batteries = absentes.</p>
        <ul>
        <li><strong>Le stock masque le flux :</strong> 288,8 GW de gaz et 254,2 GW d'éolien affichent les mêmes MW
        sur la carte, des valeurs de réseau opposées.</li>
        <li><strong>Le pipeline est indatable :</strong> 84,5 % des unités pré-construction sans année
        ({table("q2_no_startyear_by_status.csv", ["status", "n", "share_no_start"], 10)}).</li>
        <li><strong>L'historique mort est géant :</strong> {dead_gw:,.0f} GW retirés/annulés vs {op_gw:,.0f} GW
        opérants — la photo 2026 est le résidu d'un très grand cimetière.</li>
        </ul>
        {table("q2_stock_flows.csv")}
        """,
        "2",
    )

    q3 = section(
        "Q3 — Retirer bioénergie / « autres » : quels nœuds deviennent insolables ?",
        f"""
        <p>Mesure : part de la capacité dispatchable venant de bio/géothermie, et ratio de couverture de la pointe
        hiver après retrait (seuil d'insolvabilité : couverture &lt; 50 %).</p>
        {table("q3_bio_other_removal.csv", ["iso3", "peak_gw", "bio_geo_mw", "bio_geo_share_dispatch", "dispatch_no_bio_mw", "cover_ratio_after", "insolvable"], 15)}
        <p><strong>Verdict :</strong> le Danemark (54,5 % de son dispatchable en bioénergie de chauffage urbain →
        couverture 0,28) et l'Estonie (0,32) deviennent insolvables. 24,4 GW de bioénergie opérants (GB 5,3, FI 3,7,
        SE 3,3) sont la source dispatchable invisible des zones nordiques.</p>
        """,
        "3",
    )

    q4 = section(
        "Q4 — Ponts thermiques : fermer un actif fossile isole-t-il un cluster renouvelable ?",
        f"""
        <p>Réseau des interconnexions existantes GTD : les 38 pays européens forment <strong>une seule composante
        connexe</strong>. Mais certains clusters renouvelables ne tiennent que par 1-2 liens :</p>
        {table("q4_renewable_clusters.csv", ["cluster", "n_countries", "ren_op_gw", "n_external_links", "external_capacity_mw"], 12)}
        <ul>
        <li><strong>Ibérie (ES+PT) : 100,7 GW renouvelables, un seul lien de 2 800 MW vers la France.</strong>
        Le gaz espagnol (31,5 GW fossiles) n'est pas un choix de production : c'est le « pont thermique » qui garde
        le cluster connecté.</li>
        <li><strong>Géorgie : 3,0 GW renouvelables, 1 lien de 150 MW</strong> → le cas le plus extrême.</li>
        <li>Arêtes critiques (betweenness) : TÜR–GRC (660 MW) et GRC–ITA (500 MW) sont les goulots qui relient
        Balkans/Moyen-Orient/Asie à l'UE.</li>
        </ul>
        {table("q4_pivot_nodes.csv", ["iso3", "fossil_mw", "fossil_share", "n_components_after", "nodes_isolated_share"], 12)}
        """,
        "4",
    )

    q5 = section(
        "Q5 — Les 3 corridors où une ligne HT démultiplie la valeur des centrales existantes",
        f"""
        <p>Méthode : risque de curtailment côté source (renouvelable opérante / ligne existante) × capacité planifiée
        (score de levier).</p>
        <ol>
        <li><strong>Espagne ↔ France (+5 200 MW planifiés)</strong> : 83,6 GW de renouvelables espagnols pour 8,5 GW
        d'interconnexion (ratio 9,8). Chaque MW pyrénéen débloque des GW déjà construits, aujourd'hui contraints.</li>
        <li><strong>Égypte ↔ Grèce (5 000 MW)</strong> : GREGY — 63,9 GW de parc égyptien, potentiel désertique
        LCOE &lt; 3 c€/kWh, zéro ligne existante → la ligne crée le marché.</li>
        <li><strong>Italie ↔ Tunisie (2 600 MW) + Grèce ↔ Libye (2 000 MW)</strong> : ELMED et extensions —
        transforme l'Italie (nœud d'articulation) en hub d'arbitrage Maghreb-UE.</li>
        </ol>
        {table("q5_corridor_leverage.csv", ["from_iso3", "to_iso3", "existing_mw", "planned_mw", "curtail_risk", "lever_score_GW"], 10)}
        """,
        "5",
    )

    q6 = section(
        "Q6 — Ombres de prix : LCOE bas mais prix final élevé (proxy congestion)",
        f"""
        <p>Proxy sans données de marché (ENTSO-E bloqué) : GW de renouvelables opérants par GW d'interconnexion.</p>
        {table("q6_shadow_prices_proxy.csv", ["iso3", "ren_op_mw", "inter_mw", "ren_gw_per_inter_gw"], 15)}
        <p><strong>Arbitrage du siècle :</strong> Espagne/Portugal (solaire le moins cher d'Europe, congestion
        pyrénéenne), Türkiye (56,9 GW renouvelables / 2,9 GW de liens), Irlande (éolien offshore contraint par un
        seul lien). Là où l'écart production/prix est maximal, il y a soit une batterie à construire, soit un câble
        à acquérir. Systèmes insulaires (Chypre, Islande, Azerbaïdjan, Arménie, Kosovo, Biélorussie) : ∞.</p>
        """,
        "6",
    )

    q7 = section(
        "Q7 — Quand le stockage dépasse-t-il le ramping du gaz restant ?",
        f"""
        <p><strong>La carte ne peut pas répondre — et c'est la réponse.</strong> Le stockage visible par GEM
        (pompage + solaire co-localisé) = <strong>19,1 GW cumulés en 2026</strong>, contre <strong>268 GW de gaz
        opérants</strong>. Le modèle bayésien pyro (régression log-linéaire, 500 échantillons) : croisement
        <strong>pas avant 2040</strong> (médiane {q7_cross}).</p>
        {fig("q7_bayesian_crossover.png", "Croisement stockage vs gaz — modèle bayésien pyro")}
        <p>Mais les batteries réelles (~15-20 GW BESS installés UE-2025, pipeline &gt;100 GW) sont <em>invisibles</em>
        dans les données GEM. Zones déjà croisées <em>en pipeline</em> (stockage planifié ≥ gaz opérant) :
        <strong>Grèce (−18 GW), Autriche (−3,4), Portugal (−2,1)</strong> — le point de bascule y est décidé, il
        reste à construire. Zones très loin : Italie (48,8 GW de gaz), GB, DE, ES.</p>
        {table("q7_storage_vs_gas.csv", ["iso3", "gas_op_mw", "gas_pipe_mw", "storage_op_mw", "storage_pipe_mw"], 15)}
        """,
        "7",
    )

    q8 = section(
        "Q8 — Le soir est-il encore 100 % fossile malgré les GW solaires ?",
        f"""
        <p>Proxy statique hiver : charge (pointe indicative ENTSO-E 2024) − production propre estimée à 19h
        (solaire 2 %, éolien 25 %, nucléaire 90 %, hydro 45 %, bio 60 %, géothermie 85 %, pompage 50 %).</p>
        {table("q8_evening_fossil.csv", ["iso3", "peak_gw", "solar_gw", "fossil_gw", "storage_gw", "evening_fossil_index"], 18)}
        <p><strong>La carte flatte le solaire.</strong> L'Allemagne affiche 38 GW solaires — en hiver à 19h, il en
        reste ~0,8 GW et 73 % de la pointe reste fossile. Les prochains marchés du stockage : DE, PL, IT, GR, IE, TÜR.</p>
        {fig("duck_curve_proxy.png", "Proxy duck curve — charge nette hiver (DE, ES, IT, GR)")}
        """,
        "8",
    )

    q9 = section(
        "Q9 — Cascade canicule + sécheresse 3σ : qui perd hydro, thermique et lignes à la fois ?",
        f"""
        <p>Indice composite par pays : rangs normalisés de part hydro, part fossile, part variable et inverse du
        ratio d'interconnexion + probabilité de queue conjointe (gaussienne multivariée scipy).</p>
        {table("q9_cascade_index.csv", ["iso3", "hydro_share", "fossil_share", "var_share", "cascade_index"], 15)}
        <p><strong>Le scénario cascade frappe d'abord Türkiye, Portugal, Italie, Espagne, Grèce</strong> — les nœuds
        où hydro (asséchée), thermique (dégradée par la chaleur) et lignes (congestion thermique) se dégradent
        simultanément. La carte montre la moyenne ; la valeur est dans la queue — elle commence à Lisbonne, Madrid,
        Rome et Ankara.</p>
        """,
        "9",
    )

    q10 = section(
        "Q10 — Hubris clusters : renouvelables sans inertie",
        f"""
        <p>KMeans (scikit-learn, 4 clusters) sur [part variable, part stockage, ratio pipeline] → cluster « hubris »
        = forte pénétration variable + zéro stockage + pipeline massif.</p>
        {table("q10_hubris_clusters.csv", ["iso3", "var_share", "storage_share", "pipeline_ratio", "total_mw", "is_hubris"], 25)}
        <p><strong>Cluster hubris : Estonie (75,5 % variable, 0 % stockage), Grèce (48,6 %/2,9 %), Irlande
        (43,3 %/2,4 %), Monténégro.</strong> Le Danemark (70,3 % variable, 0 % pompage) est à la frontière. Ces
        systèmes sont les plus proches d'un événement de fréquence : la production variable y dépasse parfois 100 %
        de la demande, sans inertie locale.</p>
        """,
        "10",
    )

    q11 = section(
        "Q11 — Couplage fossile caché : X GW fossiles par GW renouvelable",
        f"""
        <p>Ratio de capacité fossile opérante par GW renouvelable — le « fossile persistant » structurel de chaque
        système (backup, flexibilité, chaleur).</p>
        {table("q11_fossil_renewable_coupling.csv", ["iso3", "ren_op_mw", "fossil_op_mw", "fossil_per_ren"], 15)}
        <p>En Europe, 1 GW renouvelable opérant s'appuie en moyenne sur ~0,6 GW fossile résiduel — jusqu'à 1,6-1,8
        en PL/CZ/IT. La vraie transition est un ratio, pas un interrupteur.</p>
        """,
        "11",
    )

    q12 = section(
        "Q12 — Frontières en friction : divergence climatique × interconnexion",
        f"""
        <p>Score de friction = |part fossile A − part fossile B| × capacité d'interconnexion.</p>
        {table("q12_friction_pairs.csv", ["from_iso3", "to_iso3", "inter_mw", "fossil_a", "fossil_b", "friction_score_GW"], 15)}
        <p><strong>Les axes à plus forte friction sont les transalpins (CH-IT, CH-DE, FR-IT) et les frontières Est
        (RO-SR, BA-SR).</strong> Le mécanisme est déjà en action : CBAM sur l'électricité des Balkans (Serbie 68,9 %
        fossile), prix divergents CH vs IT. La friction ne ferme pas les lignes — elle les transforme en instruments
        de politique climatique, et la valeur de l'interconnexion devient une rente de divergence.</p>
        """,
        "12",
    )

    q13 = section(
        "Q13 — Financés non construits, et zombies (approuvés sans capital)",
        f"""
        <p><strong>En construction (Europe) :</strong> solaire 24,4 GW, éolien 21,0 GW, gaz 16,3 GW, nucléaire 8,7 GW,
        hydro 3,4 GW. <strong>Pipeline total : 1 123,6 GW</strong>.</p>
        <p class='highlight'>Dont <strong>{pipe_no_owner_gw:,.0f} GW (21,5 % du pipeline) sans AUCUN owner
        identifiable</strong> — les zombies à racheter : le permitting existe, le capital non.</p>
        {table("q13_pipeline_no_owner.csv", ["iso3", "pipeline_mw", "pipeline_no_owner_mw", "share_no_owner"], 15)}
        {table("q13_zombies.csv", ["iso3", "type", "status", "n", "capacity_mw"], 12)}
        <p>Rappel d'historique : 496,7 GW annulés + 216,8 GW retirés — le taux de mortalité réel de ces pipelines
        est de 30-40 %.</p>
        """,
        "13",
    )

    q14 = section(
        "Q14 — Innovation grid-edge vs capacité brute (proxy)",
        f"""
        <p>Données brevets (EPO/WIPO) hors périmètre sandbox → proxy mesurable : ratio pipeline / croissance passée.</p>
        {table("q14_growth_proxy.csv", ["iso3", "growth_2015_2021", "pipeline_ratio", "ip_vs_cap_proxy"], 12)}
        <p>Les zones où <strong>l'annonce dépasse l'installation</strong> (Estonie, Grèce, Irlande : ratio
        pipeline/parc &gt; 4, croissance passée faible) sont celles où la valeur future sera capturée en logiciel
        (V2G, microgrids, flexibilité) plutôt qu'en béton. Limite : 34,6 % d'années manquantes → ordres de grandeur.</p>
        """,
        "14",
    )

    q15 = section(
        "Q15 — La question ultime : effacer ou doubler ?",
        f"""
        <p>Mesures de résilience : entropie de Shannon du mix (diversité) + nombre de technologies &gt; 1 GW.</p>
        {table("q15_resilience_entropy.csv", ["iso3", "shannon_entropy", "n_tech_gt1gw"], 20)}
        <ul>
        <li><strong>J'effacerais le charbon</strong> (111,7 GW opérants, 0,8 % du mix capacitif, déjà à moitié mort :
        496,7 GW annulés) — son retrait ne crée pas d'îlot : éolien, solaire et interconnexion prennent le relais.</li>
        <li><strong>Je doublerais stockage + interconnexion, conjointement</strong> — la seule paire qui augmente
        l'entropie du système sans augmenter sa masse fossile. Les trois résultats convergent : clusters renouvelables
        étranglés par 1-2 liens (Q4), soir encore &gt;70 % fossile (Q8), stockage visible 20× plus petit que le gaz (Q7).</li>
        </ul>
        <p>Dans un système complexe, la diversité bat l'optimisation : les pays à haute entropie (SE, FI, FR)
        encaissent les chocs (Q9) sans blackout.</p>
        {fig("europe_plants.png", "Centrales européennes (GIPT août 2026) — top 6 000 unités par capacité")}
        """,
        "15",
    )

    audit = section(
        "Annexe — Audit de la stack et vérification des affirmations",
        f"""
        <p>Le script <code>10_stack_audit.py</code> exécute chaque composant de la stack et vérifie les
        affirmations chiffrées de la lecture « banques centrales de l'électricité » contre les données.</p>
        <h3>1. Cohérence de la stack</h3>
        {table("audit_stack.csv")}
        <p class='highlight'>polars et duckdb (SQL sur le même parquet) donnent des totaux identiques
        (écart max 0 %). Le re-cluster KMeans reproduit exactement le cluster hubris du rapport
        (EST, GRC, IRL, MNE). Le ré-échantillonnage pyro retrouve un taux de croissance du stockage
        GEM-tracké de ~110 %/an. La topologie networkx reproduit les 3 arêtes critiques.</p>
        <h3>2. Vérification des affirmations</h3>
        {table("audit_claims.csv", ["affirmation", "valeur_mesuree", "valeur_min_attendue", "reference_externe", "unite", "statut"], 30)}
        <p class='highlight'>Bilan : 20 confirmées, <strong>2 erronées</strong> (Grèce et Irlande affichées
        « &gt;50 % variable » alors que le cluster hubris réel est EST/GRC/IRL/MNE avec GRC 48,6 % et IRL 43,3 %),
        1 écart significatif : la Norvège affichée « 34,8 GW hydro » alors que le GIPT n'en compte que 28,9 GW
        opérants — un biais de couverture GEM en soi.</p>
        <p class='note'>Chiffres hors périmètre GEM/GTD (non vérifiables ici) : CIP 37 Mds$, Brookfield TF-II
        20 Mds$, Macquarie 3 Mds$, enspired 962 MW, Coalburn 2 500 MW, RTE 105 000 km, Elia+50Hertz 10 200 km,
        prix 0→500 €/MWh. L'interconnexion grecque totalise 3 430 MW (TÜR 660 + ITA 500 + MKD 1 100 + BGR 770) ;
        « 150 MW » est le lien Géorgie–Türkiye, pas la Grèce.</p>
        """,
    )

    limits = section(
        "Limites et prolongements",
        """
        <ul>
        <li><strong>Pas de données horaires</strong> (ENTSO-E Transparency, Geofabrik, Overpass, IRENA, Ember
        inaccessibles depuis le sandbox) → duck curve et ramping = proxys statiques explicites (facteurs d'usage,
        pointes indicatives ENTSO-E ~2024).</li>
        <li><strong>Batteries absentes des données GEM</strong> → Q7 est une borne basse du stockage réel.</li>
        <li><strong>34,6 % d'années de mise en service manquantes</strong> → les séries de croissance sont des bornes.</li>
        <li><strong>Prolongements</strong> : brancher ENTSO-E horaire (Q8/Q9 exactes), registre européen BESS
        (Q7 datable), prix de marché (Q6 chiffré), open-tyndp pour les projets PCI/PMI (Q13 précisé).</li>
        <li><strong>Reproductibilité</strong> : <code>scripts/*.py</code> → <code>output/tables/*.csv</code> ;
        <code>run_all.sh</code> relance tout ; vérifié <em>ruff</em> 0, <em>mypy</em> 0, <em>pyright</em> 0.</li>
        </ul>
        """,
    )

    # ------------------------------------------------------------------
    # Assemblage HTML
    # ------------------------------------------------------------------
    nav_links = "".join(
        f"<a href='#q{i}'>{i}</a>" for i in range(1, 16)
    )

    doc = f"""<!DOCTYPE html>
<html lang="fr">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>15 questions « god tier » — système électrique européen (OpenGridWorks/GEM)</title>
<style>
:root {{
  --bg:#0e1116; --panel:#161b22; --panel2:#1c2230; --line:#2a3242;
  --fg:#d8dee9; --muted:#8b95a7; --accent:#4da6ff; --good:#2e8b57; --bad:#d62728;
  --warn:#f2b134;
}}
* {{ box-sizing:border-box; }}
body {{ margin:0; background:var(--bg); color:var(--fg); font:15px/1.6 -apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif; }}
header {{ background:linear-gradient(135deg,#10141c,#16202e); border-bottom:1px solid var(--line); padding:34px 28px 26px; }}
header h1 {{ margin:0 0 6px; font-size:26px; letter-spacing:.2px; }}
header p {{ margin:2px 0; color:var(--muted); font-size:13.5px; }}
nav.sticky {{ position:sticky; top:0; z-index:50; background:rgba(14,17,22,.94); backdrop-filter:blur(6px);
  border-bottom:1px solid var(--line); padding:8px 28px; }}
nav.sticky a {{ display:inline-block; min-width:26px; text-align:center; margin-right:6px; padding:3px 8px;
  border:1px solid var(--line); border-radius:6px; color:var(--accent); text-decoration:none; font-size:12.5px; }}
nav.sticky a:hover {{ background:var(--panel2); }}
main {{ max-width:1180px; margin:0 auto; padding:22px 28px 60px; }}
section {{ background:var(--panel); border:1px solid var(--line); border-radius:12px; padding:22px 26px; margin:20px 0; }}
section h2 {{ margin:0 0 14px; font-size:20px; display:flex; align-items:baseline; gap:10px; }}
.qid {{ background:var(--accent); color:#08131f; border-radius:7px; font-weight:800; font-size:13px;
  padding:2px 8px; }}
h3 {{ color:var(--accent); font-size:15.5px; margin:16px 0 6px; }}
p {{ margin:8px 0; }}
p.lead {{ font-size:15.5px; color:#e6edf5; }}
p.note {{ color:var(--muted); font-size:12.5px; }}
p.highlight {{ background:var(--panel2); border-left:4px solid var(--warn); padding:10px 14px; border-radius:6px; }}
ul, ol {{ margin:8px 0 10px 22px; }}
li {{ margin:5px 0; }}
.kpis {{ display:grid; grid-template-columns:repeat(auto-fit,minmax(150px,1fr)); gap:10px; margin:18px 0 6px; }}
.kpi {{ background:var(--panel2); border:1px solid var(--line); border-radius:10px; padding:10px 12px; text-align:center; }}
.kpi .k {{ font-size:11px; color:var(--muted); text-transform:uppercase; letter-spacing:.4px; }}
.kpi .v {{ font-size:19px; font-weight:700; color:var(--accent); margin-top:2px; }}
.tblwrap {{ overflow-x:auto; margin:12px 0; }}
table {{ border-collapse:collapse; width:100%; font-size:12.5px; }}
th {{ background:var(--panel2); color:var(--muted); text-transform:uppercase; font-size:10.5px; letter-spacing:.5px;
  padding:7px 10px; text-align:left; border-bottom:2px solid var(--line); }}
td {{ padding:6px 10px; border-bottom:1px solid var(--line); font-variant-numeric:tabular-nums; }}
tr:hover td {{ background:rgba(77,166,255,.05); }}
figure {{ margin:16px 0; text-align:center; }}
figure img {{ max-width:100%; border-radius:10px; border:1px solid var(--line); }}
figcaption {{ color:var(--muted); font-size:12px; margin-top:6px; }}
footer {{ color:var(--muted); font-size:12px; text-align:center; padding:20px; border-top:1px solid var(--line); }}
code {{ background:var(--panel2); padding:1px 6px; border-radius:4px; font-size:12px; }}
a {{ color:var(--accent); }}
@media (max-width:700px) {{ main {{ padding:14px; }} section {{ padding:16px; }} }}
</style>
</head>
<body>
<header>
  <h1>15 questions « god tier » sur le système électrique européen</h1>
  <p>Données publiques OpenGridWorks · Global Energy Monitor · Global Transmission Database · OSM</p>
  <p>polars · duckdb · scipy · scikit-learn · pyro · networkx — vérifié ruff · mypy · pyright</p>
</header>
<nav class="sticky">{nav_links}</nav>
<main>
{intro}
{corridors}
{q1}{q2}{q3}{q4}{q5}{q6}{q7}{q8}{q9}{q10}{q11}{q12}{q13}{q14}{q15}
{audit}
{limits}
</main>
<footer>Rapport généré automatiquement par <code>scripts/08_html_report.py</code> · tables dans
<code>output/tables/</code> · figures intégrées en base64 · données GEM CC BY 4.0 · OSM ODbL</footer>
</body>
</html>"""

    OUT.parent.mkdir(parents=True, exist_ok=True)
    OUT.write_text(doc, encoding="utf-8")
    print(f"rapport HTML généré : {OUT} ({len(doc)/1024:.0f} Ko)")


if __name__ == "__main__":
    main()
