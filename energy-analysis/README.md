# Analyse énergétique Europe — OpenGridWorks / GEM data

Analyse de données publiques sur le système électrique européen, répondant aux
15 questions « god tier » posées (corridors d'interconnexion, dynamique
temporelle, fragilité systémique, capital & souveraineté).

## Données utilisées (sources publiques)

| Jeu de données | Source | Contenu | Licence |
|---|---|---|---|
| Global Integrated Power Tracker (GIPT), août 2026 | Global Energy Monitor (`GlobalEnergyMonitor/gipt-dashboard`) | ~182 600 unités/phase de centrales monde entier : capacité, statut, technologie, années, géoloc, opérateur/owner | CC BY 4.0 |
| Global Transmission Database v1.0 (GTD) | OET / PyPSA-Earth (`open-energy-transition/electricity-transmission-database`) | Interconnexions transnationales existantes (2023) et planifiées, capacité MW par paire de pays | CC BY 4.0 |
| Global Solar Power Tracker févr. 2026 | Global Energy Monitor | Détail projets solaires | CC BY 4.0 |
| OSM power lines (haute tension) | OET MapYourGrid | Lignes HT OSM (fond de carte) | ODbL |

> Remarque d'accès : `api.opengridworks.com` n'expose pas de documentation
> publique exploitable ; les couches de la carte OpenGridWorks sont construites
> sur les trackers GEM, récupérés ici depuis les dépôts GitHub publics.
> ENTSO-E Transparency, Geofabrik et Overpass sont inaccessibles depuis le
> sandbox (réseau filtré) : les analyses « flux horaires » utilisent donc des
> proxys statiques explicites (voir le rapport pour chaque limite).

## Stack

polars, duckdb, scipy, scikit-learn, pyro-ppl, networkx, matplotlib.
Code vérifié avec ruff + mypy.

## Exécution

```bash
cd energy-analysis
python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
.venv/bin/python scripts/01_profile.py
.venv/bin/python scripts/02_corridors.py
...
```

Les données brutes (`data/raw`) et intermédiaires (`data/processed`) ne sont
pas versionnées dans git. Les tables de résultats et figures sont dans
`output/`.

## Livrables

- `docs/15_questions_europe.md` — le rapport principal (en français)
- `output/tables/*.csv` — tous les chiffres cités dans le rapport
- `output/figures/*.png` — cartes et graphiques
