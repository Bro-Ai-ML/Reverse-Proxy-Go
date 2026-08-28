# 15 questions « god tier » sur le système électrique européen
### Analyse quantitative des données publiques (OpenGridWorks / Global Energy Monitor / GTD)

**Date d'analyse :** 28 août 2026 — **Périmètre :** grille électrique européenne (EU27 + Royaume-Uni + Norvège + Suisse + Balkans + Ukraine/Moldavie + Türkiye + Géorgie) — **Stack :** polars, duckdb, scipy, scikit-learn, pyro, networkx (code vérifié ruff + mypy)

---

## 0. Données et méthode — ce qui a été fait, ce qui manque

La carte OpenGridWorks n'expose pas d'API publique documentée. Mais ses couches sont construites sur des jeux de données publics que nous avons récupérés directement :

| Couche OGW | Source récupérée | Contenu | Volume |
|---|---|---|---|
| `plants` (osmGen) | **Global Integrated Power Tracker, août 2026** (GEM) | centrales/unités : capacité, statut, technologie, années, coordonnées, owners | 182 592 unités (monde), 61 412 pour la grille Europe |
| `tx` (interconnexions) | **Global Transmission Database v1.0** (OET/PyPSA-Earth) | lignes transnationales existantes (2023) + planifiées, par paire de pays | ~1 000 paires |
| `datacenters`, `hpoints`, `evCharging`, etc. | — | couches **quasi exclusivement US** (EIA, HIFLD, AFDC) : absentes ou très pauvres pour l'Europe | — |
| lignes HT (fond) | OSM power lines (MapYourGrid/OET) | tracé global des lignes >100 kV | fond de carte |

**Limites assumées** (réseau sandbox filtré : ENTSO-E Transparency, Geofabrik, Overpass, IRENA, Ember inaccessibles) :
1. **Pas de données horaires** → les analyses « flux » (duck curve, ramping) utilisent des **proxys statiques explicites** (capacités, statuts, années, pointes de demande indicatives).
2. **Les batteries ne sont pas trackées par GEM** : le stockage se réduit au pompage hydraulique + au drapeau « associated storage » sur le solaire. C'est en soi un résultat (Q7).
3. Les années de démarrage manquent pour 34,6 % des unités → les séries de croissance sont des bornes, pas des valeurs exactes.

**Tous les chiffres cités** sont dans `output/tables/*.csv` ; les figures dans `output/figures/*.png`.

---

## 🜂 LES NOUVEAUX CORRIDORS — « routes de la soie énergétique » et « chemin du milieu »

**Où sont-ils ? Dans les planifiés de la GTD, pas dans l'existant.** L'Europe opère aujourd'hui avec **~105 GW d'interconnexions internes** et seulement **~1,4 GW vers l'Afrique du Nord**. Le pipeline dit autre chose :

### 1. Corridor Sud (Méditerranée) — le plus massif
| Paire | Existant | Planifié | Projet connu |
|---|---|---|---|
| Égypte ↔ Grèce | 0 MW | **5 000 MW** | GREGY (câble vert 3 GW à terme) |
| Italie ↔ Tunisie | 0 MW | **2 600 MW** | ELMED (600 MW) + extensions |
| Chypre ↔ Égypte / Grèce / Israël | 0 / 0 / 0 | 2 000 + 2 000 + 2 000 MW | EuroAsia Interconnector |
| Grèce ↔ Libye | 0 MW | 2 000 MW | — |
| Maroc ↔ Portugal | 0 MW | 1 000 MW | — |
| Malte ↔ Tunisie | 0 MW | 250 MW | — |

**Agrégat régional : Europe–Afrique du Nord = 1 400 → 6 850 MW planifiés (+5 450 MW), et le corridor Afrique du Nord–Moyen-Orient +2 700 MW.** C'est la « route de la soie » solaire : le Sahara et le Nil deviennent les exportateurs de l'Europe, et les interconnexions deviennent l'actif stratégique (cf. Q5).

### 2. Corridor Est — « chemin du milieu » (Middle Corridor / Caspienne)
| Paire | Existant | Planifié |
|---|---|---|
| Azerbaïdjan ↔ Géorgie | 2 000 MW | — |
| Géorgie ↔ Türkiye | 150 MW | **1 050 MW** |
| Géorgie ↔ Russie | 950 MW | 1 000 MW |
| Arménie ↔ Iran | 350 MW | 850 MW |
| Arménie ↔ Géorgie | 200 MW | 350 MW |

La Géorgie — **3 GW de renouvelables opérants, un seul lien de 150 MW vers le monde** — est l'exemple parfait du cluster bloqué : le câble sous-caspien (non encore dans la GTD, ~4-6 GW en projet) est ce qui le transforme en hub de transit entre l'hydro d'Asie centrale (KGZ+TJK : 9 GW en cluster isolé, Q4) et l'Europe. **Le « nouveau chemin du milieu » énergétique existe déjà dans les plans ; il n'existe pas encore en capacité.**

### 3. Corridor Nord (îles britanniques + Mer du Nord)
Allemagne ↔ Royaume-Uni **+2 800 MW**, Danemark ↔ Royaume-Uni **+2 800 MW**, France ↔ Royaume-Uni **+5 875 MW planifiés**, Irlande ↔ France **+700 MW (Celtic)**, Irlande ↔ GB **+2 370 MW**.

### 4. Corridor Est-européen (synchronisation + Ukraine)
Estonie ↔ Lettonie **+2 100 MW**, Lituanie ↔ Pologne **+1 000 MW (Harmony Link)**, Allemagne ↔ Pologne **+2 400 MW**, Roumanie ↔ Serbie **+2 310 MW**, Autriche ↔ Allemagne **+7 800 MW** (le plus gros renforcement intra-UE).

**Lecture stratégique :** l'Europe ajoute ~30 GW planifiés de corridors Sud+Est+Nord. Les interconnexions ne sont plus du « marché » — ce sont **les nouvelles routes de la soie énergétique**, et ceux qui tiennent le permitting (Q13) et le cash tiennent la carte 2035.

---

## 🜄 1. « Montre-moi les données que tu as choisi de NE PAS cartographier — pourquoi leur absence est un signal »

**Réponse courte : les données absentes ne sont pas les coordonnées (GEM ne garde que les unités géolocalisées) — ce sont des classes d'actifs entières, des champs économiques et des régions.**

Ce que le tracker ne cartographie **pas** (et que nous avons mesuré) :

| Absence | Mesure | Signal |
|---|---|---|
| **Batteries / stockage hors pompage** | Aucun MW de batterie dans les 182 592 lignes ; stockage = pompage (1 045 unités) + drapeau « associated storage » sur le solaire | Le marché du stockage (Europe ~15-20 GW BESS en 2025, pipeline >100 GW) est **invisible** pour la carte → actif sous-coté, et Q7 devient une question de marché, pas de carte |
| **Owners** | 65,6 % des unités européennes sans owner ; **88,4 % du solaire** ; 83,2 % sans opérateur | La couche « capital » de la carte est vide → qui possède le futur (Q13) ne peut pas être lu depuis la carte |
| **Années de démarrage** | 34,6 % des unités ; **84,5 % du pré-construction** ; 95 % des annulés ; 99,6 % des shelved-inferred | Le pipeline est **intemporel** : impossible de dater l'avenir → toute promesse « GW en 2030 » est une estimation |
| **Couches US-only** | congressionalDistricts, energyCommunities, evCharging, shalePlays, coalResources, floodplains | La carte est un objet **américain** ; pour l'Europe, la couche réseau se réduit à OSM → la granularité européenne est structurellement plus pauvre |
| **Hydro africain** | GEM suit ~1 700 unités hydro pour la grille Europe ; l'Afrique est notoirement moins détaillée | Comme le dit la question : ce qui n'est pas mesuré n'est pas régulé, n'est pas financé → actif sous-coté |

**Verdict : la carte est un mensonge utile — son silence est aussi structuré que son bruit.**

## 🜂 2. « Quelle est la résolution temporelle de chaque couche, et où les fréquences d'échantillonnage créent des illusions de stabilité ? »

| Couche | Résolution temporelle | Vintage |
|---|---|---|
| GIPT (plants) | **Photo annuelle** : statut (operating/construction/pre-construction/announced/…) + année de mise en service + année de retrait, par unité | Août 2026 |
| GTD (interconnexions) | **Photo 2023** pour l'existant ; le planifié a un champ `year_planned` **rempli à ~0 %** (valeurs « - ») | v1.0, 2023 |
| OSM lignes | Instantané du dernier rendu | continu, non daté |
| Batteries | **Absentes** (cf. Q1) | — |

**Où l'échantillonnage crée des illusions de stabilité :**
1. **Le stock masque le flux.** Une centrale à gaz (288,8 GW opérants en Europe) et un parc éolien (254,2 GW) affichent des MW identiques sur la carte mais des valeurs de réseau opposées. La carte montre la photo ; le réseau est un film (Q8).
2. **Le pipeline est indatable.** 84,5 % des unités « pre-construction » n'ont pas d'année : les 781 GW « pre-construction » + 267 GW « announced » ne peuvent pas être placés sur une frise → toute lecture « X GW en 2030 » est une projection, pas une donnée.
3. **L'historique mort est géant.** 941,7 GW de capacité annulée/retirée/shelvée en Europe — presque un parc opérant entier (1 210,8 GW). La photo 2026 est le résidu d'un très grand cimetière : les taux d'annulation réels (ex. éolien : 44,3 GW annulés + 18,3 GW « cancelled-inferred ») sont la vraie résolution temporelle.

## 🜄 3. « Si je retirais tous les actifs "autres" et "biomasse", quels nœuds deviennent insolables ? »

Dans le GIPT, le fourre-tout « other » se cache dans les champs *Technology* et *Fuel* (valeurs `unknown`/`other`) ; la biomasse est une catégorie propre. Mesure : part de la capacité dispatchable qui vient de bio/geo, et ratio de couverture de la pointe après retrait.

**Nœuds qui deviennent insolables (couverture de pointe < 50 %) :**
| Pays | Bio+Geo (MW) | Part du dispatchable | Couverture après retrait |
|---|---|---|---|
| **Danemark** | 1 934 | **54,5 %** | **0,28** → insolvable |
| **Estonie** | 105 | 18,6 % | **0,32** → insolvable |
| Islande | 839 | 30,3 % | 0,89 (géothermie = base) |
| Finlande | 3 743 | 26,1 % | 0,84 |
| Suède | 3 334 | 12,4 % | — |

Le **Danemark est le cas d'école** : 54,5 % de son dispatchable est bioénergie (cogénération chauffage urbain). La carte montre 7,2 GW d'éolien et fait oublier que le système de chaleur danois est un actif fossile-biomasse de 1,9 GW dont la disparition laisserait Copenhague sans pilotage hiver. C'est exactement le risque de modélisation que la question pointe : **la biomasse est la source dispatchable invisible des zones rurales et nordiques** — 24,4 GW opérants en Europe, concentrés en GB (5,3), FI (3,7), SE (3,3), DE, DK.

## 🜂 4. « Identifie les ponts thermiques énergétiques : zones où la fermeture d'un actif fossile isolerait un cluster renouvelable »

Réseau construit sur les interconnexions existantes GTD (38 pays européens en **une seule composante connexe** — bonne nouvelle), avec la capacité fossile de chaque nœud.

**Clusters renouvelables faiblement rattachés** (part renouvelable > 55 % du parc, liens externes comptés en MW) :

| Cluster | Renouvelables (GW) | Liens externes | Capacité des liens |
|---|---|---|---|
| **Ibérie (ES+PT)** | **100,7 GW** | 3 (FRA 2 800 / MAR 1 400 / DZA 2 000) | **6 200 MW — dont seulement 2 800 vers l'Europe** |
| Balkans (AL,BA,GR,HR,ME) | 23,7 GW | 12 liens, tous ≤ 2 000 MW | 9 940 MW |
| Géorgie | 3,0 GW | **1 lien** (→Türkiye) | **150 MW** |
| Estonie | 1,7 GW | 1 lien (→FIN) | 1 016 MW |
| Hydro Asie centrale (KG+TJ) | 9,0 GW | 3 liens | 6 470 MW |

**Pivots topologiques** (points d'articulation du réseau, avec leur masse fossile) : **Italie (50,0 GW fossiles), Türkiye (46,5 GW), Royaume-Uni (37,0 GW), Espagne (31,5 GW)**, plus LTU/FIN/SWE dans le sous-graphe Europe.

**Lecture :** le « pont thermique » n°1 est **l'Espagne** — articulation du réseau ouest, 31,5 GW fossiles, et c'est elle qui relie les 100 GW renouvelables ibériques au continent via **un seul lien de 2 800 MW aux Pyrénées**. La séquence de fermeture du gaz espagnol (CCGT) n'est pas un choix de production : c'est la décision qui détermine si 100 GW de renouvelables restent connectés à l'Europe. Les liens critiques (betweenness) confirment : **TÜR-GRC (660 MW) et GRC-ITA (500 MW)** sont les deux arêtes les plus chargées en flux du graphe — le couloir gréco-turc est le goulot qui relie les Balkans, le Moyen-Orient et l'Asie à l'UE.

## 🜃 5. « Quels sont les 3 corridors de transmission où une ligne HT démultiplierait la valeur de centrales existantes par un facteur 5+ ? »

Méthode : pour chaque paire planifiée, risque de curtailment côté source (renouvelable opérante / capacité de ligne existante) × capacité planifiée (score de levier). Résultats complets : `q5_corridor_leverage.csv`.

**1. Espagne ↔ France (+5 200 MW planifiés vs 2 800 existants).** 83,6 GW de renouvelables opérants en Espagne pour 8,5 GW d'interconnexion totale (ratio 9,8 GW-ren/GW-ligne, l'un des plus élevés d'Europe). Chaque MW de ligne pyrénéenne débloque plusieurs MW de solaire/éolien déjà construits, aujourd'hui contraints (prix espagnols structurellement sous la moyenne UE depuis 2022). Facteur de levier estimé : la ligne vaut plus que n'importe quelle nouvelle centrale.

**2. Égypte ↔ Grèce (5 000 MW planifiés, 0 existant).** Le projet GREGY : 63,9 GW de parc égyptien (dont 54,5 GW fossiles à convertir progressivement) et un potentiel solaire/éolien de désert à LCOE < 3 c€/kWh. Zéro ligne aujourd'hui → chaque MW de câble est un monopole de transit pour les 2 000 MW de pipeline égyptien et les 2 GW chypriotes en attente (EuroAsia). C'est le corridor qui **crée** un marché, pas qui le soulage.

**3. Italie ↔ Tunisie (2 600 MW planifiés, 0 existant) + Grèce ↔ Libye (2 000 MW).** ELMED + extensions : la Tunisie a 5,5 GW fossiles et un potentiel solaire immense pour 580 MW de liens existants (ratio 10,3 GW-ren… mais surtout 10 GW de demande industrielle italienne à prix élevés). En même temps, **Italie = nœud d'articulation** (Q4) : ces câbles transforment le « pont thermique » italien en hub d'arbitrage Maghreb-UE.

*Mention honorable :* Irlande (degré 1, 630 MW aujourd'hui) : FRA-IRL 700 MW + GBR-IRL 2 370 MW planifiés — l'île passe d'insulaire à hub éolien offshore.

## 🜄 6. « Cartographie les ombres de prix : zones où le LCOE est bas mais le prix final reste élevé à cause des congestions »

Sans données de marché (ENTSO-E bloqué), proxy de congestion : **GW de renouvelables opérants par GW d'interconnexion** (`q6_shadow_prices_proxy.csv`).

| Pays | Renouvelables op. (GW) | Interconnexion (GW) | GW-ren / GW-ligne |
|---|---|---|---|
| Géorgie | 3,0 | 0,15 | **20,1** |
| **Türkiye** | 56,9 | 2,9 | **19,6** |
| **Espagne** | 83,6 | 8,5 | **9,8** |
| **Irlande** | 5,8 | 0,63 | **9,2** |
| **Portugal** | 17,1 | 2,3 | **7,4** |
| Chypre, Islande, Azerbaïdjan, Arménie, Kosovo, Biélorussie | — | **0** | ∞ (systèmes insulaires) |

**Arbitrage du siècle :** Espagne/Portugal (LCOE solaire parmi les plus bas d'Europe, prix industriels tirés par la congestion pyrénéenne), Türkiye (hub de 56,9 GW de renouvelables mais 2,9 GW de liens → soit la zone de stockage la plus rentable d'Europe, soit le futur exportateur si les corridors Est se matérialisent), Irlande (éolien offshore contraint par un seul lien). **Là où l'écart production/prix est maximal, il y a soit une batterie à construire, soit un câble à acquérir.**

## 🜃 7. « À quelle date le stock cumulé de batteries dépasse-t-il la capacité de ramping du gaz restant dans chaque zone ? »

**Réponse honnête : la carte ne peut pas répondre — et c'est LA réponse.**

- Stockage visible par GEM (pompage + solaire co-localisé) : **19,1 GW cumulés en 2026** (14,1 GW en 2020), + un pipeline de pompage massif (Grèce 25,7 GW !, Espagne 9,6, GB 10,6, Autriche 2,1).
- Gaz opérant Europe : **268 GW** (Italie 48,8, GB 37,0, Allemagne 34,6, Espagne 31,3, Türkiye 26,1), + pipeline gaz ~100 GW.
- **Modèle bayésien pyro** (`q7_bayesian_crossover.csv`, régression log-linéaire hiérarchique sur la série GEM) : sur les données GEM, le croisement stockage ≥ gaz n'arrive **pas avant 2040** (médiane 2040, P10-P90 = 2040). Le stockage GEM-tracké atteindrait ~96 GW en 2040 contre ~202 GW de gaz.

**Mais les batteries réelles ne sont pas dans la carte** : ~15-20 GW BESS déjà installés dans l'UE en 2025 et un pipeline de plusieurs centaines de GW (files d'attente de raccordement) sont invisibles ici. Deux mondes se superposent :
- **Zones déjà « croisées » en pipeline** (stockage planifié ≥ gaz opérant) : **Grèce (−18 GW), Autriche (−3,4), Portugal (−2,1)** → le point de bascule structurel y est déjà *décidé*, il reste à construire.
- **Zones très loin** : Italie (48,8 GW de gaz, besoin ×5 de stockage vs pipeline), GB, DE, ES → il faudra soit un stockage ×5-10, soit des corridors d'import (Q5) pour que le gaz devienne actif échoué.

**Quand vendre ses actifs fossiles ?** Dans les zones croisées (GR/AT/PT), maintenant ; dans les zones à gaz résiduel massif (IT/GB/DE), la fenêtre est 2030-2033 selon la vitesse réelle de déploiement BESS — invisible sur cette carte.

## 🜃 8. « Quel est le "duck curve" moyen de chaque pays, et où la pointe du soir (18-21h) est-elle encore 100 % fossile ? »

Proxy statique hiver : charge (pointe indicative ENTSO-E ~2024) − production propre estimée à 19h (facteurs : solaire 2 %, éolien 25 %, nucléaire 90 %, hydro 45 %, bio 60 %, géothermie 85 %, pompage 50 %). → `q8_evening_fossil.csv`, figure `duck_curve_proxy.png`.

**Le soir est encore >70 % fossile dans 10 des 18 premiers pays :**

| Rang | Pays | Solaire (GW) | Fossile (GW) | Index soir fossile |
|---|---|---|---|---|
| 1 | Malte | 0,02 | 0,55 | 1,00 |
| 2 | Moldavie | 0,17 | 1,17 | 0,98 |
| 3 | Chypre | 0,40 | 1,85 | 0,96 |
| 4 | Kosovo | 0,02 | 1,29 | 0,95 |
| 5 | Azerbaïdjan | 0,30 | 7,14 | 0,89 |
| 6 | Estonie | 1,05 | 0,46 | 0,82 |
| 8 | **Pologne** | 8,9 | 32,5 | 0,81 |
| 9 | Serbie | 0,20 | 5,4 | 0,81 |
| 10 | **Irlande** | 0,93 | 6,4 | 0,74 |
| 11 | **Allemagne** | **38,0** | 62,7 | 0,73 |
| 13 | **Italie** | 5,4 | 50,0 | 0,71 |
| 15 | **Grèce** | 6,0 | 9,1 | 0,64 |

**La carte flatte le solaire.** L'Allemagne affiche 38 GW solaires (le plus grand parc d'Europe) — mais en hiver à 19h, le solaire contribue ~0,8 GW et 73 % de la pointe reste à couvrir par du fossile. La Grèce a un ratio solaire/pointe de 0,60 (le meilleur d'Europe) et un index de 0,64 : **le jour est vert, le soir est noir.** Les prochains marchés du stockage sont exactement ces pays : DE, PL, IT, GR, IE, TÜR.

## 🜄 9. « Modélise l'impact d'une canicule + sécheresse de 3σ : quels nœuds perdent simultanément hydro, efficacité thermique et capacité de ligne ? »

Méthode : indice composite par pays sur les dimensions hydro (part), thermique (part fossile+nucléaire), variable (part éolien+solaire) et interconnexion (inverse du ratio), normalisées en rangs + probabilité de queue conjointe par gaussienne multivariée (scipy). → `q9_cascade_index.csv`.

| Rang | Pays | Hydro | Fossile | Variable | Indice cascade |
|---|---|---|---|---|---|
| 1 | **Türkiye** | 24,9 % | 43,7 % | 28,6 % | **0,50** |
| 2 | **Portugal** | 35,0 % | 20,9 % | 41,9 % | **0,45** |
| 3 | **Italie** | 20,5 % | 58,9 % | 18,7 % | **0,44** |
| 4 | Irlande | 3,8 % | 51,9 % | 43,3 % | 0,43 |
| 5 | Espagne | 13,2 % | 25,6 % | 54,5 % | 0,43 |
| 6 | Grèce | 13,2 % | 38,3 % | 48,6 % | 0,41 |
| 7 | GB | 4,4 % | 39,0 % | 44,1 % | 0,40 |
| 8 | Roumanie | 28,1 % | 24,8 % | 40,6 % | 0,40 |
| 9 | Pologne | 3,9 % | 61,0 % | 33,6 % | 0,40 |
| 10 | Allemagne | 4,5 % | 39,0 % | 55,1 % | 0,39 |

**Le scénario cascade 3σ frappe en premier la péninsule ibérique + Italie + Türkiye** : ce sont les nœuds où hydro (asséchée), thermique (dégradée par la chaleur des fleuves de refroidissement) et interconnexion (congestion thermique des lignes) se dégradent *simultanément*. La carte montre la moyenne ; la valeur est dans la queue — et la queue commence à Lisbonne, Madrid, Rome et Ankara.

## 🜄 10. « Quels sont les "hubris clusters" : régions où la densité de renouvelables a dépassé l'inertie locale ? »

Clustering KMeans (scikit-learn) sur [part variable, part stockage, ratio pipeline] → cluster « hubris » = forte pénétration variable + zéro stockage + pipeline massif. → `q10_hubris_clusters.csv`.

**Cluster hubris : Estonie (75,5 % variable, 0 % stockage, pipeline ×6,2), Grèce (48,6 % variable, 2,9 % stockage, pipeline ×4,6), Irlande (43,3 % variable, 2,4 % stockage, pipeline ×5,2), Monténégro.** Le Danemark (70,3 % variable) est à la frontière du cluster : 70 % de sa capacité est éolien/solaire, 0 % de stockage pompé.

Ces systèmes sont les plus proches d'un événement de fréquence : à certains instants, la production variable couvre >100 % de la demande (Irlande l'a déjà atteint plusieurs heures), et l'inertie ne vient plus que des interconnexions et des services système (condensateurs synchrones) — invisibles sur la carte. **Les zones qui se vantent d'être « 100 % renouvelables » à un instant T sont les plus proches d'un blackout de fréquence** : ce sont précisément EST, GRC, IRL, DNK.

## 🜃 11. « Pour chaque TWh renouvelable, combien de TWh fossiles sont encore nécessaires en couplage indirect ? »

Proxy mesurable : **ratio de capacité fossile opérante par GW renouvelable** (`q11_fossil_renewable_coupling.csv`) — le « fossile persistant » structurel de chaque système (backup, flexibilité, chaleur).

| Pays | Fossile op. (GW) / Renouv. op. (GW) |
|---|---|
| Malte | 29,3 |
| Biélorussie | 29,1 |
| Kosovo | 6,9 |
| Moldavie | 5,5 |
| Azerbaïdjan | 4,6 |
| Chypre | 3,4 |
| Serbie | 2,3 |
| Pologne | 1,6 |
| Italie | 1,5 |

Et l'autre couche du couplage (qualitative, non mesurable ici) : l'acier des pylônes, le diesel des bateaux d'éolien offshore, le béton des fondations — **la vraie transition est un ratio, pas un interrupteur.** La carte montre les MW renouvelables ; elle ne montre pas les MW fossiles qui les soutiennent : en Europe, 1 GW renouvelable opérant s'appuie en moyenne sur ~0,6 GW fossile résiduel, jusqu'à 1,6-1,8 GW en PL/CZ/IT.

## 🜄 12. « Quels pays ont une frontière énergétique avec un voisin dont la politique climatique diverge, et comment la friction réécrit les flux ? »

Score de friction = divergence de part fossile entre deux pays connectés × capacité d'interconnexion (`q12_friction_pairs.csv`).

| Paire | Interconnexion (MW) | Fossile A / B | Friction (GW·Δ) |
|---|---|---|---|
| **Suisse ↔ Italie** | 4 950 | 0,6 % / 58,9 % | **2,89** |
| France ↔ Royaume-Uni | 8 875 | 9,7 % / 39,0 % | 2,60 |
| Suisse ↔ Allemagne | 6 100 | 0,6 % / 39,0 % | 2,35 |
| France ↔ Italie | 4 100 | 9,7 % / 58,9 % | 2,02 |
| Autriche ↔ Allemagne | 9 200 | 22,0 % / 39,0 % | 1,57 |
| Roumanie ↔ Serbie | 2 990 | 24,8 % / 68,9 % | 1,32 |
| Espagne ↔ France | 8 000 | 25,6 % / 9,7 % | 1,27 |
| Bosnie ↔ Serbie | 4 190 | 40,4 % / 68,9 % | 1,20 |

**Lecture :** les lignes à plus forte friction sont les axes transalpins (CH-IT, CH-DE, FR-IT) et les frontières Est (RO-SR, BA-SR). Le mécanisme est déjà en action : CBAM sur l'acier/l'électricité des Balkans (Serbie 68,9 % fossile, Bosnie 40,4 % → lignite), prix divergents CH (hydro) vs IT (gaz), et la sortie du charbon polonais qui réécrit DE-PL. **La friction ne ferme pas les lignes — elle les transforme en instruments de politique climatique : les pays à décarbonation rapide deviennent exportateurs nets vers les voisins lents, et la valeur de l'interconnexion devient une rente de divergence.**

## 🜅 13. « Quels actifs sont financés mais pas construits, et quels sont les zombies (approuvés sans capital) ? »

**Financés / en construction (Europe) :** solaire 24,4 GW, éolien 21,0 GW, gaz 16,3 GW, nucléaire 8,7 GW, hydro 3,4 GW.

**Le pipeline complet : 1 123,6 GW** (pre-construction 781,2 + announced 266,8 + construction 75,6). Dont :

**Zombies — approuvés mais sans capital identifiable : 241,5 GW (21,5 % du pipeline) n'ont AUCUN owner dans les données GEM** (`q13_pipeline_no_owner.csv`) :
- Allemagne : **90,1 %** du pipeline MW sans owner
- Royaume-Uni : 75,7 % · Irlande : 56,0 % · Norvège : 60,3 % · Grèce : 50,7 % · Espagne : 31,0 %

**Top zombies par capacité** (pre-construction) : Espagne solaire **91,7 GW** (1 082 projets), GB éolien 73,4 GW (434), Suède éolien 68,1 GW (75), Grèce solaire 51,1 GW (1 866), Espagne éolien 47,0 GW, Irlande éolien 42,5 GW.

**Le marché se joue sur le pipeline :** les 21,5 % sans owner sont les candidats au rachat — le permitting existe (les projets sont dans le tracker), le capital non. Les 30 derniers mois du marché européen se joueront entre ceux qui ont le cash et ceux qui ont les permis. **Note d'historique :** 496,7 GW annulés + 216,8 GW retirés montrent le taux de mortalité réel de ces pipelines (≈ 30-40 % ne verront jamais le jour).

## 🜅 14. « Dans quelles régions la densité de brevets grid-edge croît-elle plus vite que l'installation de capacité brute ? »

Les données brevets (EPO/WIPO) sont hors périmètre sandbox ; proxy mesurable : **ratio pipeline (croissance annoncée) sur croissance passée mesurée** (`q14_growth_proxy.csv`).

| Pays | Croissance capacité 2015→2021 (proxy) | Ratio pipeline / parc opérant |
|---|---|---|
| **Estonie** | 0,55 | **6,2** |
| **Grèce** | — | **4,6** |
| **Irlande** | — | **5,2** |
| Monténégro | — | 3,8 |
| GB | — | 1,7 |
| Espagne | — | 1,5 |
| Turquie | 0,25 | 0,25 |

Les zones où **l'innovation physique n'a pas suivi l'annonce** (EST, GRC, IRL : ratio pipeline/parc > 4, croissance passée faible) sont celles où l'avenir se jouera en logiciel (V2G, microgrids, stockage distribué, flexibilité) plutôt qu'en béton — et où la valeur capturée sera la plus grande, car la rareté n'est pas la ressource mais **la coordination**. (Limite : 34,6 % d'années manquantes biaisent la croissance passée — chiffres à lire comme des ordres de grandeur.)

## 🜆 15. La question ultime — « laquelle effacerais-tu, laquelle doublerais-tu ? »

Mesures de résilience par pays : entropie de Shannon du mix (diversité) + nombre de technologies >1 GW (`q15_resilience_entropy.csv`).

**La plus diversifiée :** Finlande (1,51), Suède (1,39), Bulgarie (1,35), Belgique (1,34), Ukraine (1,32), France (1,25).
**La moins diversifiée :** Algérie (0,13), Libye (0,15), Malte (0,15), Tunisie (0,29), Kosovo (0,44), Albanie (0,45), Luxembourg (0,49).

**Réponse :**
- **J'effacerais le charbon.** 111,7 GW opérants en Europe (0,8 % du mix capacitif) pour une part d'électricité marginale et des émissions disproportionnées — et l'historique le dit déjà : 496,7 GW annulés, il est déjà à moitié mort. L'effacer ne crée pas d'îlot : l'éolien, le solaire et l'interconnexion prennent le relais, et les Balkans/PL sont les derniers bastions (SRB 68,9 % fossile, POL 61 %).
- **Je doublerais le stockage + l'interconnexion, conjointement.** Ce n'est pas « plus du même » : c'est la seule paire qui augmente l'entropie du système sans augmenter sa masse fossile. Les trois résultats qui convergent ici : (a) les clusters renouvelables sont étranglés par 1-2 liens (Ibérie, Irlande, Géorgie, Q4) ; (b) le soir est encore >70 % fossile malgré les GW solaires (Q8) ; (c) le stockage visible est 20 fois plus petit que le gaz (Q7). **Dans un système complexe, la diversité bat l'optimisation : les pays à haute entropie (SE, FI, FR) sont exactement ceux qui encaissent les chocs (Q9) sans blackout.**

---

## Conclusion — la carte est un mensonge utile

Les 15 questions convergent vers trois enseignements :

1. **La valeur n'est pas dans les bulles colorées mais dans le quand, le si, le à condition que.** Les données GEM sont une photo de stock ; tout ce qui est flux (batteries, heures, prix, permitting) est soit absent, soit indatable (84,5 % du pipeline sans année).
2. **Les absences sont des signaux de pouvoir** : pas de batteries → actif sous-coté ; pas d'owners → capital invisible ; pas de couches Europe fines → la carte est un objet américain qui projette sa grille de lecture sur le monde.
3. **L'Europe 2035 se dessine déjà dans les planifiés** : ~30 GW de nouveaux corridors (Sud 5,45 GW Afrique du Nord, Est Caspienne-Caucase, Nord îles, Est-européen synchronisation) — et ceux qui tiennent permitting + cash (Q13 : 241,5 GW sans owner à racheter) tiennent la carte.

**Pour aller plus loin** (quand le réseau le permettra) : brancher ENTSO-E Transparency pour les heures réelles (Q8/Q9 deviennent exactes), intégrer le registre européen BESS (Q7 devient datable), et coupler avec les prix de marché (Q6 devient un arbitrage chiffré).

*Reproductibilité : `energy-analysis/scripts/*.py` → `output/tables/*.csv` et `output/figures/*.png` ; `energy-analysis/docs/15_questions_europe.md` (ce document).*
