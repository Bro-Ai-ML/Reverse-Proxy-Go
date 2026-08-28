#!/usr/bin/env python
"""Génère le rapport PDF (docs/rapport_europe.pdf) à partir des tables.

Pure Python (reportlab) — aucune dépendance système. Police DejaVuSans embarquée
pour le support Unicode complet. Page de garde + TOC + 17 sections + tableaux
+ figures + numéros de page.
"""

from __future__ import annotations

from pathlib import Path

import polars as pl
from reportlab.lib import colors
from reportlab.lib.enums import TA_CENTER, TA_JUSTIFY, TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen.canvas import Canvas
from reportlab.platypus import (  # noqa: PLC0415
    BaseDocTemplate,
    Flowable,
    Frame,
    Image,
    NextPageTemplate,
    PageBreak,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)
from reportlab.platypus.tableofcontents import TableOfContents

ROOT = Path(__file__).resolve().parents[1]
TABLES = ROOT / "output" / "tables"
FIGURES = ROOT / "output" / "figures"
OUT = ROOT / "docs" / "rapport_europe.pdf"

# ---------------------------------------------------------------------------
# Polices (Unicode complet)
# ---------------------------------------------------------------------------
FONT_DIR = Path("/usr/share/fonts/truetype/dejavu")
pdfmetrics.registerFont(TTFont("DejaVu", str(FONT_DIR / "DejaVuSans.ttf")))
pdfmetrics.registerFont(TTFont("DejaVu-Bold", str(FONT_DIR / "DejaVuSans-Bold.ttf")))
pdfmetrics.registerFont(TTFont("DejaVuMono", str(FONT_DIR / "DejaVuSansMono.ttf")))

# Palette (version claire, adaptée PDF/impression)
INK = colors.HexColor("#0e1116")
BLUE = colors.HexColor("#1666c1")
BLUE_LIGHT = colors.HexColor("#eaf1fb")
GRAY = colors.HexColor("#5a6472")
LINE = colors.HexColor("#d5dce5")
GREEN = colors.HexColor("#2e8b57")
RED = colors.HexColor("#b3271c")
AMBER = colors.HexColor("#b47d0e")
PANEL = colors.HexColor("#f4f6fa")


def _st(name: str, **kw: object) -> ParagraphStyle:
    base: dict[str, object] = dict(
        fontName="DejaVu",
        fontSize=9.5,
        leading=14,
        textColor=INK,
        alignment=TA_JUSTIFY,
        spaceAfter=6,
    )
    base.update(kw)  # pyright: ignore[reportCallIssue, reportArgumentType]
    return ParagraphStyle(name, **base)  # pyright: ignore[reportArgumentType]


ST_H1 = _st("h1", fontName="DejaVu-Bold", fontSize=21, leading=27, spaceAfter=10, alignment=TA_LEFT)
ST_H2 = _st("h2", fontName="DejaVu-Bold", fontSize=14.5, leading=19, spaceBefore=14, spaceAfter=7, textColor=BLUE)
ST_H3 = _st("h3", fontName="DejaVu-Bold", fontSize=11, leading=15, spaceBefore=10, spaceAfter=5)
ST_P = _st("p")
ST_P_SMALL = _st("psmall", fontSize=8.5, leading=12.5, textColor=GRAY)
ST_BULLET = _st("bullet", leftIndent=14, bulletIndent=4, spaceAfter=4)
ST_TBL_CAPTION = _st("tcap", fontSize=8, leading=11, textColor=GRAY, alignment=TA_LEFT, spaceBefore=4, spaceAfter=2)
ST_KPI_K = _st("kpik", fontName="DejaVu-Bold", fontSize=7.5, leading=10, textColor=GRAY, alignment=TA_CENTER, spaceAfter=0)
ST_KPI_V = _st("kpiv", fontName="DejaVu-Bold", fontSize=15, leading=19, textColor=BLUE, alignment=TA_CENTER, spaceAfter=0)
ST_COVER_TITLE = _st("ct", fontName="DejaVu-Bold", fontSize=26, leading=33, textColor=colors.white, alignment=TA_LEFT, spaceAfter=8)
ST_COVER_SUB = _st("cs", fontSize=12, leading=17, textColor=colors.HexColor("#cdd9e8"), alignment=TA_LEFT, spaceAfter=4)


def _fmt(v: object) -> str:
    if v is None:
        return "—"
    if isinstance(v, float):
        return f"{v:,.1f}"
    return str(v)


def _csv(name: str, cols: list[str] | None = None, n: int | None = None) -> pl.DataFrame:
    d = pl.read_csv(TABLES / name)
    if cols:
        d = d.select(cols)
    if n:
        d = d.head(n)
    return d


def _num_table(name: str, cols: list[str] | None = None, n: int | None = None,
               caption: str | None = None, max_w: float = 500.0) -> list[Flowable]:
    """Tableau depuis un CSV avec en-têtes stylés."""
    d = _csv(name, cols, n)
    data = [[Paragraph(f"<b>{c}</b>", _st("th", fontName="DejaVu-Bold", fontSize=7.6, leading=9.5,
                                           textColor=colors.white, alignment=TA_LEFT, spaceAfter=0))
             for c in d.columns]]
    for r in d.iter_rows(named=True):
        data.append([Paragraph(str(_fmt(r[c])), _st("td", fontSize=8, leading=10.5, alignment=TA_LEFT, spaceAfter=0))
                     for c in d.columns])
    colw = [max_w / len(d.columns)] * len(d.columns)
    t = Table(data, colWidths=colw, repeatRows=1, hAlign="LEFT")
    t.setStyle(TableStyle([
        ("BACKGROUND", (0, 0), (-1, 0), BLUE),
        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, PANEL]),
        ("GRID", (0, 0), (-1, -1), 0.4, LINE),
        ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
        ("TOPPADDING", (0, 0), (-1, -1), 2.5),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 2.5),
        ("LEFTPADDING", (0, 0), (-1, -1), 5),
        ("RIGHTPADDING", (0, 0), (-1, -1), 5),
    ]))
    out: list[Flowable] = [t]
    if caption:
        out.append(Paragraph(caption, ST_TBL_CAPTION))
        out.append(Spacer(1, 8))
    return out


def _fig(name: str, caption: str, max_w: float = 460.0) -> list[Flowable]:
    p = FIGURES / name
    if not p.exists():
        return []
    from PIL import Image as PILImage

    with PILImage.open(p) as im:
        w, h = im.size
    scale = min(1.0, max_w / w)
    out: list[Flowable] = [
        Image(str(p), width=w * scale, height=h * scale),
        Paragraph(caption, ST_TBL_CAPTION),
        Spacer(1, 8),
    ]
    return out


def _bullets(items: list[str]) -> list[Flowable]:
    return [Paragraph(f"• {it}", ST_BULLET) for it in items]


def _kpis(pairs: list[tuple[str, str]]) -> Table:
    rows = []
    for i in range(0, len(pairs), 4):
        chunk = pairs[i : i + 4]
        k_row = [Paragraph(k, ST_KPI_K) for k, _ in chunk]
        v_row = [Paragraph(v, ST_KPI_V) for v, _ in chunk]
        rows.append(k_row)
        rows.append(v_row)
    t = Table(rows, colWidths=[121] * 4)
    t.setStyle(TableStyle([
        ("BOX", (0, 0), (-1, -1), 0.8, LINE),
        ("INNERGRID", (0, 0), (-1, -1), 0.5, LINE),
        ("BACKGROUND", (0, 0), (-1, -1), PANEL),
        ("TOPPADDING", (0, 0), (-1, -1), 5),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
    ]))
    return t


class ReportDoc(BaseDocTemplate):
    """Document avec TOC multi-build et bookmarks par section."""

    def __init__(self, filename: str) -> None:
        super().__init__(
            filename,
            pagesize=A4,
            leftMargin=20 * mm,
            rightMargin=20 * mm,
            topMargin=16 * mm,
            bottomMargin=18 * mm,
            title="15 questions god tier — système électrique européen",
            author="energy-analysis (OpenGridWorks / GEM)",
        )
        frame = Frame(self.leftMargin, self.bottomMargin, self.width, self.height, id="main")
        cover = Frame(20 * mm, 18 * mm, A4[0] - 40 * mm, A4[1] - 34 * mm, id="cover")
        self.addPageTemplates([
            PageTemplate(id="cover", frames=[cover], onPage=self._cover_bg),
            PageTemplate(id="body", frames=[frame], onPage=self._page_bg),
        ])

    # -- fonds -----------------------------------------------------------
    def _cover_bg(self, canv: Canvas, doc: BaseDocTemplate) -> None:
        canv.saveState()
        canv.setFillColor(INK)
        canv.rect(0, 0, A4[0], A4[1], stroke=0, fill=1)
        canv.setFillColor(BLUE)
        canv.rect(0, A4[1] - 6 * mm, A4[0], 6 * mm, stroke=0, fill=1)
        canv.restoreState()

    def _page_bg(self, canv: Canvas, doc: BaseDocTemplate) -> None:
        canv.saveState()
        canv.setStrokeColor(BLUE)
        canv.setLineWidth(2.4)
        canv.line(0, A4[1] - 10 * mm, A4[0], A4[1] - 10 * mm)
        canv.setFont("DejaVu", 8)
        canv.setFillColor(GRAY)
        canv.drawRightString(A4[0] - 20 * mm, 10 * mm, f"Page {canv.getPageNumber()}")
        canv.drawString(20 * mm, 10 * mm, "15 questions god tier — système électrique européen")
        canv.restoreState()

    # -- TOC -------------------------------------------------------------
    def afterFlowable(self, flowable: object) -> None:
        if isinstance(flowable, Paragraph):
            style = flowable.style.name
            if style == "h2":
                text = flowable.getPlainText()
                key = f"h2-{text}"
                self.canv.bookmarkPage(key)
                self.canv.addOutlineEntry(text, key, level=0, closed=False)
                self.notify("TOCEntry", (0, text, self.page, key))


def build() -> None:
    doc = ReportDoc(str(OUT))
    story: list[Flowable] = []

    # ------------------------------------------------------------------
    # PAGE DE GARDE
    # ------------------------------------------------------------------
    story.append(NextPageTemplate("body"))
    story.append(Paragraph("15 questions « god tier »", ST_COVER_TITLE))
    story.append(Paragraph("sur le système électrique européen", ST_COVER_TITLE))
    story.append(Spacer(1, 10))
    story.append(Paragraph("Données publiques OpenGridWorks · Global Energy Monitor · Global Transmission Database · OSM", ST_COVER_SUB))
    story.append(Paragraph("polars · duckdb · scipy · scikit-learn · pyro · networkx — vérifié ruff · mypy · pyright", ST_COVER_SUB))
    story.append(Paragraph("28 août 2026 · Périmètre : EU27 + UK + Norvège + Suisse + Balkans + Ukraine/Moldavie + Türkiye + Géorgie", ST_COVER_SUB))
    story.append(Spacer(1, 26))

    df_op = _csv("operating_by_type_europe.csv")
    op_gw = float(df_op["capacity_mw"].sum() or 0.0) / 1e3
    df_q2 = _csv("q2_stock_flows.csv")
    pipe_gw = float(df_q2["pipeline_cap_mw"].first() or 0.0) / 1e3  # type: ignore[arg-type]
    dead_gw = float(df_q2["dead_cap_mw"].first() or 0.0) / 1e3  # type: ignore[arg-type]
    df_q13 = _csv("q13_pipeline_no_owner.csv")
    no_owner_gw = float(df_q13["pipeline_no_owner_mw"].sum() or 0.0) / 1e3  # type: ignore[arg-type]
    df_q1 = _csv("q1_missingness_europe.csv")
    no_owner_pct = float(df_q1.filter(pl.col("indicator") == "units_without_owner")["share"].first() or 0.0) * 100  # type: ignore[arg-type]
    no_start_pct = float(df_q1.filter(pl.col("indicator") == "units_without_start_year")["share"].first() or 0.0) * 100  # type: ignore[arg-type]

    story.append(_kpis([
        ("Parc opérant Europe", f"{op_gw:,.0f} GW"),
        ("Pipeline annoncé", f"{pipe_gw:,.0f} GW"),
        ("Capacité morte", f"{dead_gw:,.0f} GW"),
        ("Pipeline sans owner", f"{no_owner_gw:,.0f} GW"),
        ("Unités sans owner", f"{no_owner_pct:.0f} %"),
        ("Sans année MES", f"{no_start_pct:.0f} %"),
        ("Stockage visible (2026)", "19,1 GW"),
        ("Gaz opérant", "268 GW"),
    ]))
    story.append(Spacer(1, 16))
    story.append(Paragraph(
        "<b>Résumé.</b> La carte OpenGridWorks n'expose pas d'API documentée ; ses couches sont construites sur "
        "des jeux publics que cette analyse récupère directement : Global Integrated Power Tracker (GEM, août 2026, "
        "182 592 unités mondiales), Global Transmission Database v1.0 (interconnexions existantes 2023 + planifiées), "
        "Global Solar Power Tracker et lignes haute tension OSM. Le rapport répond aux 15 questions « god tier » : "
        "corridors transnationaux (routes de la soie énergétique), biais de couverture, topologie (ponts thermiques), "
        "dynamique temporelle (stockage vs gaz), fragilité systémique (cascade, hubris), capital et souveraineté "
        "(zombies, friction). Tous les chiffres proviennent des tables <font face='DejaVuMono' size='8'>output/tables/*.csv</font>.",
        _st("psum", fontSize=10, leading=15, textColor=colors.HexColor("#dde6f1")),
    ))
    story.append(Paragraph(
        "Limites : pas de données horaires (ENTSO-E, Geofabrik, Overpass inaccessibles depuis le sandbox) → "
        "duck curve et ramping = proxys statiques explicites ; batteries absentes des données GEM → le stockage "
        "visible se limite au pompage + solaire co-localisé ; 34,6 % des unités sans année de mise en service.",
        _st("plimit", fontSize=8.5, leading=12.5, textColor=colors.HexColor("#9fb0c4")),
    ))
    story.append(PageBreak())

    # ------------------------------------------------------------------
    # TABLE DES MATIÈRES
    # ------------------------------------------------------------------
    story.append(Paragraph("Table des matières", ST_H1))
    toc = TableOfContents()
    toc.levelStyles = [
        ParagraphStyle("toc0", fontName="DejaVu-Bold", fontSize=10.5, leading=17, textColor=INK),
    ]
    story.append(toc)
    story.append(PageBreak())

    # ------------------------------------------------------------------
    # CORRIDORS
    # ------------------------------------------------------------------
    story.append(Paragraph("Les nouveaux corridors — routes de la soie énergétique et chemin du milieu", ST_H2))
    story.append(Paragraph(
        "L'Europe opère aujourd'hui avec ~105 GW d'interconnexions internes et seulement ~1,4 GW vers l'Afrique "
        "du Nord. Le pipeline (GTD v1.0) raconte une autre histoire : <b>~30 GW de nouvelles lignes planifiées</b>, "
        "dont les plus massives sont des corridors transnationaux nouveaux.", ST_P))
    story += _num_table("corridors_new_only.csv",
                        ["from_iso3", "to_iso3", "existing_mw", "planned_mw"], 18,
                        "Nouveaux corridors planifiés (0 MW existant → capacité planifiée, MW).")
    story.append(Paragraph("Corridor Sud (Méditerranée)", ST_H3))
    story += _bullets([
        "<b>Égypte–Grèce +5 000 MW</b> (GREGY) · <b>Italie–Tunisie +2 600 MW</b> (ELMED) · "
        "<b>Chypre–Égypte / Israël / Grèce +2 000 MW</b> chacun (EuroAsia) · Grèce–Libye +2 000 MW · "
        "Maroc–Portugal +1 000 MW · Malte–Tunisie +250 MW. Agrégat Europe–Afrique du Nord : 1 400 → 6 850 MW planifiés.",
    ])
    story.append(Paragraph("Corridor Est (« chemin du milieu »)", ST_H3))
    story += _bullets([
        "Géorgie–Türkiye +1 050 MW · Géorgie–Russie +1 000 MW · Arménie–Iran +850 MW. "
        "La Géorgie (3 GW renouvelables opérants, un seul lien de 150 MW) est le symbole du corridor encore bloqué.",
    ])
    story.append(Paragraph("Corridor Nord et Est-européen", ST_H3))
    story += _bullets([
        "DE–GB +2 800 MW · DK–GB +2 800 MW · FR–GB +5 875 MW · IE–FR +700 MW (Celtic) · IE–GB +2 370 MW.",
        "EE–LV +2 100 MW · LT–PL +1 000 MW (Harmony Link) · DE–PL +2 400 MW · RO–RS +2 310 MW · "
        "AT–DE +7 800 MW (le plus gros renforcement intra-UE).",
    ])
    story += _fig("corridors_planned.png", "Corridors d'interconnexion planifiés (GTD v1.0) — capacité en GW.")

    # ------------------------------------------------------------------
    # Q1–Q15
    # ------------------------------------------------------------------
    story.append(Paragraph("Q1 — Les données NON cartographiées, et pourquoi leur absence est un signal", ST_H2))
    story.append(Paragraph(
        "Les absences ne portent pas sur les coordonnées (GEM ne garde que les unités géolocalisées) mais sur des "
        "<b>classes d'actifs entières, des champs économiques et des régions</b> :", ST_P))
    story += _bullets([
        "<b>Batteries / stockage hors pompage : aucun MW</b> dans les 182 592 lignes. Le stockage GEM = pompage "
        "(1 045 unités) + drapeau « associated storage » sur le solaire → le marché BESS européen est invisible, actif sous-coté.",
        "<b>Owners : 65,6 % des unités sans owner</b> (88,4 % du solaire) ; 83,2 % sans opérateur → la couche « capital » est vide.",
        "<b>Années de mise en service : 34,6 % manquantes</b> (84,5 % du pré-construction) → le pipeline est indatable.",
        "<b>Couches US-only</b> (congressionalDistricts, energyCommunities, evCharging, shalePlays…) : "
        "la granularité européenne de la carte est structurellement plus pauvre.",
    ])
    story += _num_table("q1_missingness_europe.csv", None, None, "Taux de données manquantes sur la grille Europe.")
    story += _num_table("q1_missingness_by_type.csv",
                        ["type", "n", "share_no_owner", "share_no_startyear"], 8,
                        "Données manquantes par type d'actif (parts).")

    story.append(Paragraph("Q2 — Résolution temporelle des couches, illusions de stabilité", ST_H2))
    story.append(Paragraph(
        "<b>Résolutions :</b> GIPT = photo annuelle (statut + année MES + retrait par unité, vintage août 2026) · "
        "GTD = photo 2023 (planifié avec <i>year_planned</i> rempli à ~0 %) · OSM = instantané non daté · "
        "Batteries = absentes.", ST_P))
    story += _bullets([
        "<b>Le stock masque le flux :</b> 288,8 GW de gaz et 254,2 GW d'éolien affichent les mêmes MW sur la "
        "carte, des valeurs de réseau opposées.",
        "<b>Le pipeline est indatable :</b> 84,5 % des unités pré-construction sans année.",
        f"<b>L'historique mort est géant :</b> {dead_gw:,.0f} GW retirés/annulés vs {op_gw:,.0f} GW opérants — "
        "la photo 2026 est le résidu d'un très grand cimetière.",
    ])
    story += _num_table("q2_no_startyear_by_status.csv", ["status", "n", "share_no_start"], 10,
                        "Part d'unités sans année de mise en service par statut.")
    story += _num_table("q2_stock_flows.csv", None, None, "Stock / flux (MW) sur la grille Europe.")

    story.append(Paragraph("Q3 — Retirer bioénergie et « autres » : quels nœuds deviennent insolables ?", ST_H2))
    story.append(Paragraph(
        "Mesure : part de la capacité dispatchable venant de bio/géothermie, et ratio de couverture de la pointe "
        "hiver après retrait (seuil d'insolvabilité : couverture &lt; 50 %).", ST_P))
    story += _num_table("q3_bio_other_removal.csv",
                        ["iso3", "peak_gw", "bio_geo_mw", "bio_geo_share_dispatch", "dispatch_no_bio_mw",
                         "cover_ratio_after", "insolvable"], 15,
                        "Dépendance à bio/géothermie et couverture de pointe après retrait (MW).")
    story.append(Paragraph(
        "<b>Verdict :</b> le Danemark (54,5 % de son dispatchable en bioénergie de chauffage urbain → couverture "
        "0,28) et l'Estonie (0,32) deviennent insolvables. 24,4 GW de bioénergie opérants (GB 5,3, FI 3,7, SE 3,3) "
        "sont la source dispatchable invisible des zones nordiques.", ST_P))

    story.append(Paragraph("Q4 — Ponts thermiques : fermer un actif fossile isole-t-il un cluster renouvelable ?", ST_H2))
    story.append(Paragraph(
        "Réseau des interconnexions existantes GTD : les 38 pays européens forment une seule composante connexe. "
        "Mais certains clusters renouvelables ne tiennent que par 1-2 liens :", ST_P))
    story += _num_table("q4_renewable_clusters.csv",
                        ["cluster", "n_countries", "ren_op_gw", "n_external_links", "external_capacity_mw"], 12,
                        "Clusters renouvelables (&gt;55 % du parc) et leurs liens externes.")
    story += _bullets([
        "<b>Ibérie (ES+PT) : 100,7 GW renouvelables, un seul lien de 2 800 MW vers la France.</b> "
        "Le gaz espagnol (31,5 GW fossiles) est le « pont thermique » qui garde le cluster connecté.",
        "<b>Géorgie : 3,0 GW renouvelables, 1 lien de 150 MW</b> → le cas le plus extrême.",
        "Arêtes critiques (betweenness) : TÜR–GRC (660 MW) et GRC–ITA (500 MW) relient Balkans/Moyen-Orient/Asie à l'UE.",
    ])
    story += _num_table("q4_pivot_nodes.csv",
                        ["iso3", "fossil_mw", "fossil_share", "n_components_after", "nodes_isolated_share"], 12,
                        "Pivots topologiques (points d'articulation) et masse fossile.")

    story.append(Paragraph("Q5 — Les 3 corridors où une ligne HT démultiplie la valeur des centrales existantes", ST_H2))
    story.append(Paragraph(
        "Méthode : risque de curtailment côté source (renouvelable opérante / ligne existante) × capacité planifiée "
        "(score de levier).", ST_P))
    story.append(Paragraph("1. Espagne ↔ France (+5 200 MW planifiés)", ST_H3))
    story.append(Paragraph(
        "83,6 GW de renouvelables espagnols pour 8,5 GW d'interconnexion (ratio 9,8). Chaque MW pyrénéen débloque "
        "des GW déjà construits, aujourd'hui contraints.", ST_P))
    story.append(Paragraph("2. Égypte ↔ Grèce (5 000 MW)", ST_H3))
    story.append(Paragraph(
        "GREGY : 63,9 GW de parc égyptien, potentiel désertique LCOE &lt; 3 c€/kWh, zéro ligne existante → "
        "la ligne crée le marché.", ST_P))
    story.append(Paragraph("3. Italie ↔ Tunisie (2 600 MW) + Grèce ↔ Libye (2 000 MW)", ST_H3))
    story.append(Paragraph(
        "ELMED et extensions : transforme l'Italie (nœud d'articulation) en hub d'arbitrage Maghreb-UE.", ST_P))
    story += _num_table("q5_corridor_leverage.csv",
                        ["from_iso3", "to_iso3", "existing_mw", "planned_mw", "curtail_risk", "lever_score_GW"], 10,
                        "Score de levier des corridors planifiés.")

    story.append(Paragraph("Q6 — Ombres de prix : LCOE bas mais prix final élevé (proxy congestion)", ST_H2))
    story.append(Paragraph(
        "Proxy sans données de marché (ENTSO-E bloqué) : GW de renouvelables opérants par GW d'interconnexion.", ST_P))
    story += _num_table("q6_shadow_prices_proxy.csv",
                        ["iso3", "ren_op_mw", "inter_mw", "ren_gw_per_inter_gw"], 15,
                        "Ratio renouvelables / interconnexion (GW par GW de ligne).")
    story.append(Paragraph(
        "<b>Arbitrage du siècle :</b> Espagne/Portugal (solaire le moins cher d'Europe, congestion pyrénéenne), "
        "Türkiye (56,9 GW renouvelables / 2,9 GW de liens), Irlande (éolien offshore contraint par un seul lien). "
        "Là où l'écart production/prix est maximal, il y a soit une batterie à construire, soit un câble à acquérir.", ST_P))

    story.append(Paragraph("Q7 — Quand le stockage dépasse-t-il le ramping du gaz restant ?", ST_H2))
    story.append(Paragraph(
        "<b>La carte ne peut pas répondre — et c'est la réponse.</b> Le stockage visible par GEM "
        "(pompage + solaire co-localisé) = <b>19,1 GW cumulés en 2026</b>, contre <b>268 GW de gaz opérants</b>. "
        "Le modèle bayésien pyro (régression log-linéaire, 500 échantillons) : croisement <b>pas avant 2040</b>.", ST_P))
    story += _fig("q7_bayesian_crossover.png", "Croisement stockage vs gaz — modèle bayésien pyro.")
    story.append(Paragraph(
        "Mais les batteries réelles (~15-20 GW BESS installés UE-2025, pipeline &gt;100 GW) sont invisibles dans "
        "les données GEM. Zones déjà croisées en pipeline (stockage planifié ≥ gaz opérant) : "
        "<b>Grèce (−18 GW), Autriche (−3,4), Portugal (−2,1)</b>. Zones très loin : Italie (48,8 GW de gaz), GB, DE, ES.", ST_P))
    story += _num_table("q7_storage_vs_gas.csv",
                        ["iso3", "gas_op_mw", "gas_pipe_mw", "storage_op_mw", "storage_pipe_mw"], 15,
                        "Gaz et stockage (MW), opérant et planifié.")

    story.append(Paragraph("Q8 — Le soir est-il encore 100 % fossile malgré les GW solaires ?", ST_H2))
    story.append(Paragraph(
        "Proxy statique hiver : charge (pointe indicative ENTSO-E 2024) − production propre estimée à 19h "
        "(solaire 2 %, éolien 25 %, nucléaire 90 %, hydro 45 %, bio 60 %, géothermie 85 %, pompage 50 %).", ST_P))
    story += _num_table("q8_evening_fossil.csv",
                        ["iso3", "peak_gw", "solar_gw", "fossil_gw", "storage_gw", "evening_fossil_index"], 18,
                        "Index « soir fossile » : part de la pointe du soir restant à couvrir (0-1).")
    story.append(Paragraph(
        "<b>La carte flatte le solaire.</b> L'Allemagne affiche 38 GW solaires — en hiver à 19h, il en reste "
        "~0,8 GW et 73 % de la pointe reste fossile. Les prochains marchés du stockage : DE, PL, IT, GR, IE, TÜR.", ST_P))
    story += _fig("duck_curve_proxy.png", "Proxy duck curve — charge nette hiver (DE, ES, IT, GR).")

    story.append(Paragraph("Q9 — Cascade canicule + sécheresse 3σ : qui perd hydro, thermique et lignes à la fois ?", ST_H2))
    story.append(Paragraph(
        "Indice composite par pays : rangs normalisés de part hydro, part fossile, part variable et inverse du "
        "ratio d'interconnexion + probabilité de queue conjointe (gaussienne multivariée scipy).", ST_P))
    story += _num_table("q9_cascade_index.csv",
                        ["iso3", "hydro_share", "fossil_share", "var_share", "cascade_index"], 15,
                        "Indice de cascade (0-1, 1 = risque maximal).")
    story.append(Paragraph(
        "<b>Le scénario cascade frappe d'abord Türkiye, Portugal, Italie, Espagne, Grèce</b> — les nœuds où "
        "hydro (asséchée), thermique (dégradée par la chaleur) et lignes (congestion thermique) se dégradent "
        "simultanément. La carte montre la moyenne ; la valeur est dans la queue.", ST_P))

    story.append(Paragraph("Q10 — Hubris clusters : renouvelables sans inertie", ST_H2))
    story.append(Paragraph(
        "KMeans (scikit-learn, 4 clusters) sur [part variable, part stockage, ratio pipeline] → cluster "
        "« hubris » = forte pénétration variable + zéro stockage + pipeline massif.", ST_P))
    story += _num_table("q10_hubris_clusters.csv",
                        ["iso3", "var_share", "storage_share", "pipeline_ratio", "total_mw", "is_hubris"], 25,
                        "Clusters hubris (is_hubris = vrai).")
    story.append(Paragraph(
        "<b>Cluster hubris : Estonie (75,5 % variable, 0 % stockage), Grèce (48,6 % / 2,9 %), Irlande "
        "(43,3 % / 2,4 %), Monténégro.</b> Le Danemark (70,3 % variable, 0 % pompage) est à la frontière. Ces "
        "systèmes sont les plus proches d'un événement de fréquence.", ST_P))

    story.append(Paragraph("Q11 — Couplage fossile caché : X GW fossiles par GW renouvelable", ST_H2))
    story.append(Paragraph(
        "Ratio de capacité fossile opérante par GW renouvelable — le « fossile persistant » structurel de chaque "
        "système (backup, flexibilité, chaleur).", ST_P))
    story += _num_table("q11_fossil_renewable_coupling.csv",
                        ["iso3", "ren_op_mw", "fossil_op_mw", "fossil_per_ren"], 15,
                        "Fossile opérant par GW renouvelable.")
    story.append(Paragraph(
        "En Europe, 1 GW renouvelable opérant s'appuie en moyenne sur ~0,6 GW fossile résiduel — jusqu'à 1,6-1,8 "
        "en PL/CZ/IT. La vraie transition est un ratio, pas un interrupteur.", ST_P))

    story.append(Paragraph("Q12 — Frontières en friction : divergence climatique × interconnexion", ST_H2))
    story.append(Paragraph(
        "Score de friction = |part fossile A − part fossile B| × capacité d'interconnexion.", ST_P))
    story += _num_table("q12_friction_pairs.csv",
                        ["from_iso3", "to_iso3", "inter_mw", "fossil_a", "fossil_b", "friction_score_GW"], 15,
                        "Paires en friction (GW·Δ).")
    story.append(Paragraph(
        "<b>Les axes à plus forte friction sont les transalpins (CH-IT, CH-DE, FR-IT) et les frontières Est "
        "(RO-SR, BA-SR).</b> CBAM sur l'électricité des Balkans (Serbie 68,9 % fossile), prix divergents CH vs IT. "
        "La friction ne ferme pas les lignes — elle les transforme en instruments de politique climatique.", ST_P))

    story.append(Paragraph("Q13 — Financés non construits, et zombies (approuvés sans capital)", ST_H2))
    story.append(Paragraph(
        "<b>En construction (Europe) :</b> solaire 24,4 GW, éolien 21,0 GW, gaz 16,3 GW, nucléaire 8,7 GW, "
        "hydro 3,4 GW. <b>Pipeline total : 1 123,6 GW</b>. Dont <b>241,5 GW (21,5 % du pipeline) sans AUCUN "
        "owner identifiable</b> — les zombies à racheter : le permitting existe, le capital non.", ST_P))
    story += _num_table("q13_pipeline_no_owner.csv",
                        ["iso3", "pipeline_mw", "pipeline_no_owner_mw", "share_no_owner"], 15,
                        "Pipeline et part sans owner par pays.")
    story += _num_table("q13_zombies.csv",
                        ["iso3", "type", "status", "n", "capacity_mw"], 12,
                        "Top projets « zombies » (annoncés / pré-construction, MW).")
    story.append(Paragraph(
        "Rappel d'historique : 496,7 GW annulés + 216,8 GW retirés — le taux de mortalité réel de ces pipelines "
        "est de 30-40 %.", ST_P))

    story.append(Paragraph("Q14 — Innovation grid-edge vs capacité brute (proxy)", ST_H2))
    story.append(Paragraph(
        "Données brevets (EPO/WIPO) hors périmètre sandbox → proxy mesurable : ratio pipeline / croissance passée.", ST_P))
    story += _num_table("q14_growth_proxy.csv",
                        ["iso3", "growth_2015_2021", "pipeline_ratio", "ip_vs_cap_proxy"], 12,
                        "Croissance passée vs pipeline.")
    story.append(Paragraph(
        "Les zones où l'annonce dépasse l'installation (Estonie, Grèce, Irlande : ratio pipeline/parc &gt; 4, "
        "croissance passée faible) sont celles où la valeur future sera capturée en logiciel (V2G, microgrids, "
        "flexibilité) plutôt qu'en béton. Limite : 34,6 % d'années manquantes → ordres de grandeur.", ST_P))

    story.append(Paragraph("Q15 — La question ultime : effacer ou doubler ?", ST_H2))
    story.append(Paragraph(
        "Mesures de résilience : entropie de Shannon du mix (diversité) + nombre de technologies &gt; 1 GW.", ST_P))
    story += _num_table("q15_resilience_entropy.csv", ["iso3", "shannon_entropy", "n_tech_gt1gw"], 20,
                        "Entropie de diversité du mix par pays.")
    story += _bullets([
        "<b>J'effacerais le charbon</b> (111,7 GW opérants, 0,8 % du mix capacitif, déjà à moitié mort : "
        "496,7 GW annulés) — son retrait ne crée pas d'îlot : éolien, solaire et interconnexion prennent le relais.",
        "<b>Je doublerais stockage + interconnexion, conjointement</b> — la seule paire qui augmente l'entropie "
        "du système sans augmenter sa masse fossile. Les trois résultats convergent : clusters renouvelables "
        "étranglés par 1-2 liens (Q4), soir encore &gt;70 % fossile (Q8), stockage visible 20× plus petit que le gaz (Q7).",
    ])
    story += _fig("europe_plants.png", "Centrales européennes (GIPT août 2026) — top 6 000 unités par capacité.")

    # ------------------------------------------------------------------
    # LIMITES
    # ------------------------------------------------------------------
    story.append(Paragraph("Limites et prolongements", ST_H2))
    story += _bullets([
        "<b>Pas de données horaires</b> (ENTSO-E Transparency, Geofabrik, Overpass, IRENA, Ember inaccessibles "
        "depuis le sandbox) → duck curve et ramping = proxys statiques explicites.",
        "<b>Batteries absentes des données GEM</b> → Q7 est une borne basse du stockage réel.",
        "<b>34,6 % d'années de mise en service manquantes</b> → les séries de croissance sont des bornes.",
        "<b>Prolongements :</b> brancher ENTSO-E horaire (Q8/Q9 exactes), registre européen BESS (Q7 datable), "
        "prix de marché (Q6 chiffré), open-tyndp pour les projets PCI/PMI (Q13 précisé).",
        "<b>Reproductibilité :</b> <font face='DejaVuMono' size='8'>scripts/*.py</font> → "
        "<font face='DejaVuMono' size='8'>output/tables/*.csv</font> ; <font face='DejaVuMono' size='8'>run_all.sh</font> "
        "relance tout ; vérifié ruff 0 · mypy 0 · pyright 0.",
    ])
    story.append(Spacer(1, 12))
    story.append(Paragraph(
        "Données : Global Energy Monitor (CC BY 4.0) · Open Energy Transition / PyPSA-Earth GTD (CC BY 4.0) · "
        "OpenStreetMap (ODbL). Rapport généré par scripts/09_pdf_report.py.", ST_P_SMALL))

    doc.multiBuild(story)
    print(f"PDF généré : {OUT} ({OUT.stat().st_size / 1024:.0f} Ko)")


if __name__ == "__main__":
    build()
