#!/usr/bin/env python
"""Génère docs/lecture_corrigee.pdf depuis docs/lecture_corrigee.md.

Mini-renderer Markdown → reportlab (sous-ensemble contrôlé : titres, tableaux,
listes, citations, gras, code inline). Police DejaVuSans embarquée.
"""

from __future__ import annotations

import re
from pathlib import Path

from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen.canvas import Canvas
from reportlab.platypus import (
    BaseDocTemplate,
    Frame,
    PageTemplate,
    Paragraph,
    Spacer,
    Table,
    TableStyle,
)

ROOT = Path(__file__).resolve().parents[1]
MD = ROOT / "docs" / "lecture_corrigee.md"
OUT = ROOT / "docs" / "lecture_corrigee.pdf"

FONT_DIR = Path("/usr/share/fonts/truetype/dejavu")
pdfmetrics.registerFont(TTFont("DejaVu", str(FONT_DIR / "DejaVuSans.ttf")))
pdfmetrics.registerFont(TTFont("DejaVu-Bold", str(FONT_DIR / "DejaVuSans-Bold.ttf")))
pdfmetrics.registerFont(TTFont("DejaVuMono", str(FONT_DIR / "DejaVuSansMono.ttf")))

INK = colors.HexColor("#0e1116")
BLUE = colors.HexColor("#1666c1")
GRAY = colors.HexColor("#5a6472")
LINE = colors.HexColor("#d5dce5")
PANEL = colors.HexColor("#f4f6fa")
RED = colors.HexColor("#b3271c")
GREEN = colors.HexColor("#2e8b57")


def _st(name: str, **kw: object) -> ParagraphStyle:
    base: dict[str, object] = dict(
        fontName="DejaVu", fontSize=9.5, leading=14.5, textColor=INK,
        alignment=TA_LEFT, spaceAfter=6,
    )
    base.update(kw)  # pyright: ignore[reportCallIssue, reportArgumentType]
    return ParagraphStyle(name, **base)  # pyright: ignore[reportArgumentType]


ST_H1 = _st("h1", fontName="DejaVu-Bold", fontSize=17, leading=22, spaceAfter=8, textColor=BLUE)
ST_H2 = _st("h2", fontName="DejaVu-Bold", fontSize=12.5, leading=17, spaceBefore=12, spaceAfter=6, textColor=BLUE)
ST_H3 = _st("h3", fontName="DejaVu-Bold", fontSize=10.5, leading=14, spaceBefore=8, spaceAfter=4)
ST_P = _st("p")
ST_BULLET = _st("b", leftIndent=12, bulletIndent=2, spaceAfter=3)
ST_QUOTE = _st("q", leftIndent=16, textColor=colors.HexColor("#444c56"), fontSize=9.5, spaceBefore=4, spaceAfter=8)
ST_TBL = _st("tblcap", fontSize=8, leading=11, textColor=GRAY, spaceAfter=2)


def _inline(text: str) -> str:
    """Convertit **bold** et `code` inline en balises reportlab."""
    text = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", text)
    text = re.sub(r"`([^`]+)`", r'<font face="DejaVuMono" size="8">\1</font>', text)
    text = text.replace("&", "&amp;").replace("<b>", "\x00B").replace("</b>", "\x00/B")
    text = re.sub(r"<font face=\"DejaVuMono\" size=\"8\">", "\x00M", text)
    text = text.replace("</font>", "\x00/M")
    text = text.replace("<", "&lt;").replace(">", "&gt;")
    text = text.replace("\x00B", "<b>").replace("\x00/B", "</b>")
    text = text.replace("\x00M", '<font face="DejaVuMono" size="8">').replace("\x00/M", "</font>")
    return text


def _md_table(lines: list[str]) -> Table:
    """Convertit des lignes '| a | b |' en Table reportlab (2e ligne = séparateur)."""
    rows: list[list[Paragraph]] = []
    for ln in lines:
        cells = [c.strip() for c in ln.strip().strip("|").split("|")]
        if all(re.fullmatch(r":?-{2,}:?", c) for c in cells if c):
            continue  # ligne séparateur
        rows.append([Paragraph(_inline(c), _st("td", fontSize=8, leading=10.5, spaceAfter=0)) for c in cells])
    ncol = max(len(r) for r in rows)
    for r in rows:
        while len(r) < ncol:
            r.append(Paragraph("", _st("td", fontSize=8, leading=10.5, spaceAfter=0)))
    t = Table(rows, colWidths=[495.0 / ncol] * ncol, repeatRows=1, hAlign="LEFT")
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
    return t


def _page_bg(canv: Canvas, doc: BaseDocTemplate) -> None:
    canv.saveState()
    canv.setStrokeColor(BLUE)
    canv.setLineWidth(2.4)
    canv.line(0, A4[1] - 10 * mm, A4[0], A4[1] - 10 * mm)
    canv.setFont("DejaVu", 8)
    canv.setFillColor(GRAY)
    canv.drawRightString(A4[0] - 20 * mm, 10 * mm, f"Page {canv.getPageNumber()}")
    canv.drawString(20 * mm, 10 * mm, "Les banques centrales de l'électricité — version corrigée")
    canv.restoreState()


def main() -> None:
    text = MD.read_text(encoding="utf-8")
    doc = BaseDocTemplate(
        str(OUT), pagesize=A4,
        leftMargin=20 * mm, rightMargin=20 * mm, topMargin=16 * mm, bottomMargin=18 * mm,
        title="Les banques centrales de l'électricité — version corrigée",
        author="energy-analysis (OpenGridWorks / GEM)",
    )
    frame = Frame(doc.leftMargin, doc.bottomMargin, doc.width, doc.height, id="main")
    doc.addPageTemplates([PageTemplate(id="body", frames=[frame], onPage=_page_bg)])

    story: list[object] = []
    lines = text.splitlines()
    i = 0
    while i < len(lines):
        ln = lines[i]
        if not ln.strip():
            i += 1
            continue
        if ln.strip() == "---":
            story.append(Spacer(1, 8))
            i += 1
            continue
        if ln.startswith("### "):
            story.append(Paragraph(_inline(ln[4:]), ST_H3))
            i += 1
            continue
        if ln.startswith("## "):
            story.append(Paragraph(_inline(ln[3:]), ST_H2))
            i += 1
            continue
        if ln.startswith("# "):
            story.append(Paragraph(_inline(ln[2:]), ST_H1))
            i += 1
            continue
        if ln.startswith("> "):
            story.append(Paragraph(_inline(ln[2:]), ST_QUOTE))
            i += 1
            continue
        if ln.startswith("- "):
            story.append(Paragraph(_inline(ln[2:]), ST_BULLET, bulletText="•"))
            i += 1
            continue
        if ln.startswith("|"):
            block = [ln]
            j = i + 1
            while j < len(lines) and lines[j].strip().startswith("|"):
                block.append(lines[j])
                j += 1
            story.append(_md_table(block))
            story.append(Spacer(1, 8))
            i = j
            continue
        # paragraphe (peut être multi-ligne)
        para = [ln]
        j = i + 1
        while j < len(lines) and lines[j].strip() and not lines[j].startswith(("#", "|", "- ", "> ", "---")):
            para.append(lines[j])
            j += 1
        story.append(Paragraph(_inline(" ".join(para)), ST_P))
        i = j

    doc.build(story)  # type: ignore[arg-type]
    print(f"PDF généré : {OUT} ({OUT.stat().st_size / 1024:.0f} Ko)")


if __name__ == "__main__":
    main()
