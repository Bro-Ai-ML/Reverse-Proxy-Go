#!/usr/bin/env bash
# Relance complète du pipeline d'analyse (données GEM + GTD → tables + figures + rapport).
set -euo pipefail
cd "$(dirname "$0")"

PY=./.venv/bin/python
if [ ! -x "$PY" ]; then
  echo "venv manquant — lancez : python3 -m venv .venv && .venv/bin/pip install -r requirements.txt"
  exit 1
fi

for script in 01_profile 02_corridors 03_topology 04_temporal 05_models 06_maps 07_misc_questions 08_html_report 09_pdf_report 10_stack_audit 11_norway_deepdive 12_lecture_corrigee_pdf; do
  echo "▶ scripts/${script}.py"
  "$PY" "scripts/${script}.py"
done

echo "✔ Terminé — tables dans output/tables/, figures dans output/figures/, rapports dans docs/ (markdown + HTML + PDF + lecture corrigée)"
