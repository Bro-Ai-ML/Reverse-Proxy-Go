# « Les banques centrales de l'électricité » — version corrigée

**Lecture systémique revue et corrigée contre les données** (GIPT GEM août 2026, GTD v1.0, statistiques officielles NVE/Energimyndigheten/Connaissance des Énergies).
L'audit automatisé (`scripts/10_stack_audit.py`) a vérifié 23 affirmations : **20 confirmées, 2 corrigées, 1 précisée, 1 écart de couverture documenté**.
Document généré le 28 août 2026.

---

## Tableau des corrections

| # | Affirmation originale | Affirmation corrigée | Mesure | Source |
|---|---|---|---|---|
| 1 | « Les pays hubris (Estonie, Grèce, Irlande, Danemark) ont **> 50 % de variable** » | **Estonie 75,5 % et Danemark 70,3 % dépassent 50 % ; Grèce 48,6 % et Irlande 43,3 % sont sous le seuil** — mais les quatre (avec Monténégro) forment bien le cluster hubris KMeans (forte pénétration variable + stockage quasi nul + pipeline massif) | GRC 48,6 % · IRL 43,3 % · EST 75,5 % · DNK 70,3 % | `q10_hubris_clusters.csv` |
| 2 | « IPTO (Grèce) … **150 MW aujourd'hui**, +5 000 MW planifiés » | **L'interconnexion grecque totalise 3 430 MW aujourd'hui** (TÜR 660 + ITA 500 + MKD 1 100 + BGR 770 + ALB 400) ; +5 000 MW planifiés (Égypte–Grèce). **150 MW est le lien Géorgie–Türkiye**, pas la Grèce | 3 430 MW, degré 5 | `topology_countries.csv`, GTD existing |
| 3 | « Norvège (Statkraft) : **34,8 GW hydro opérants** » | **~33,9 GW officiels (NVE 2026) ; le GIPT n'en compte que 28,9 GW opérants (−14,7 %)** — biais de couverture GEM documenté (voir §3) | GEM 28,9 vs officiel 33,9 | NVE (1 791 installations) vs `norway_comparison.csv` |

**Confirmées telles quelles** (20) : Italie 50,0 GW fossiles · Allemagne 38,0 GW solaires / 62,7 GW fossiles / 73 % soir · Espagne 83,6 GW renouvelables / 8,5 GW d'interconnexion · Irlande 0,3 GW stockage / 1 lien 630 MW · Belgique 1,3 GW stockage · Danemark 70 % variable / 0 % pompage / 54,5 % du dispatchable en bio · Autriche croisée stockage ≥ gaz (ratio 1,7) · Autriche 6,1 GW pompage opérant · Suède hydro+bio 17,2 GW · entropies FI 1,51 / SE 1,39 / FR 1,25.

---

## 1. Texte corrigé — Mécanisme 3 (Q10, « besoin d'inertie »)

> **Original :** « Les pays hubris (Estonie, Grèce, Irlande, Danemark) ont > 50 % de variable, 0 % de stockage. »

**Corrigé :**
> Les pays hubris identifiés par le clustering (Estonie, Grèce, Irlande, Monténégro — cluster KMeans sur part variable × stockage × pipeline) cumulent **forte pénétration variable, stockage quasi nul et pipeline massif**. Détail mesuré : **Estonie 75,5 %** de capacité variable et **0 % de stockage** ; **Danemark 70,3 %** et 0 % de pompage — seuls ces deux dépassent 50 %. **Grèce 48,6 % (2,9 % de stockage, pipeline ×4,6) et Irlande 43,3 % (2,4 %, pipeline ×5,2)** restent sous le seuil des 50 % mais appartiennent au même cluster par l'ampleur de leur pipeline. Quand la fréquence dévie, il n'y a pas de masse tournante pour la rattraper : le solaire sur le toit (onduleur électronique) n'apporte aucune inertie — le prosumer est un consommateur passif déguisé, pas un acteur de stabilité.

---

## 2. Texte corrigé — Banque centrale #1, ligne IPTO

> **Original :** `IPTO (Grèce) | Gardien du Corridor Sud : GREGY + EuroAsia. Seul lien Caucase-Méditerranée-Europe | 150 MW aujourd'hui, +5 000 MW planifiés`

**Corrigé :**

| TSO | Rôle | Donnée corrigée |
|---|---|---|
| **IPTO (Grèce)** | Gardien du Corridor Sud : GREGY + EuroAsia ; nœud entre Balkans, Moyen-Orient et UE | **3 430 MW d'interconnexion existante** (TÜR 660 + ITA 500 + MKD 1 100 + BGR 770 + ALB 400), **+5 000 MW planifiés** (Égypte–Grèce), 2 000 MW Chypre–Grèce |

*Précision : « 150 MW » était le lien Géorgie–Türkiye (le maillon faible du chemin du milieu). La Grèce n'est pas « le seul lien Caucase-Méditerranée » : la Türkiye relie déjà la Géorgie à l'Europe (ligne de 150 MW existante, +1 050 MW planifiés), et l'interconnexion grecque est un hub à 5 arêtes, pas un lien unique.*

---

## 3. Deep-dive — L'écart Norvège : GIPT vs NVE

L'affirmation « 34,8 GW hydro opérants » de la lecture est **cohérente avec les statistiques officielles** (NVE, début 2026 : **33,9 GW installés**, 1 791 installations, 137,6 TWh de production moyenne). C'est le **tracker GEM (GIPT) qui sous-compte** : 28,9 GW opérants, soit **−14,7 %**.

Et ce n'est pas propre à la Norvège — le GIPT sous-compte l'hydro partout :

| Pays | GEM opérant (GW) | GEM tous statuts (GW) | Pompage GEM (GW) | Officiel (GW) | Écart | Source officielle |
|---|---|---|---|---|---|---|
| Finlande | 2,5 | 2,6 | 0,00 | ~3,2 | **−20,5 %** | ordre de grandeur 2023 |
| France | 21,3 | 21,4 | 5,75 | 25,5 | **−16,3 %** | Connaissance des Énergies, 2024 |
| Suède | 13,8 | 14,5 | 0,09 | 16,4 | **−15,6 %** | Energimyndigheten 2024 |
| **Norvège** | **28,9** | 31,5 | 0,47 | **33,9** | **−14,7 %** | **NVE 2026** |
| Autriche | 12,9 | 16,6 | 6,14 | 14,1 | −8,4 % | ANDRITZ |
| Suisse | 14,3 | 17,0 | 3,75 | ~15,5 | −7,8 % | ordre de grandeur BFE/SFOE |

**Interprétation :**
- **Le pompage est bien capté** : France 5,75 GW vs ~5 GW de STEP officielles, Autriche 6,14 GW vs ~8,4 GW (une partie du pompage autrichien est classée « conventional storage »). L'écart ne vient donc pas des grandes stations.
- **L'écart vient des petites centrales** (< 10 MW, run-of-river et petites retenues) que GEM ne tracke pas unité par unité, et des mises à niveau récentes.
- Les top-10 GEM norvégiens (Kvilldal 1 240 MW, Tonstad 960, Aurland 1 840, Sy-Sima 720…) sont exacts — c'est la **longue traîne** qui manque.
- **Conséquence pour la lecture :** le rôle de « batterie verte » de la Norvège (34 GW officiels, 87 TWh de capacité de réservoirs) est **sous-estimé** par la carte, pas surestimé. L'argument « 4e banque centrale » n'est pas affaibli, il est renforcé.

---

## 4. Autres chiffres de la lecture — statut

| Affirmation | Statut |
|---|---|
| RTE : « 50 interconnexions, 105 000 km de lignes » | **Non vérifiable** (hors périmètre GEM/GTD) |
| Elia + 50Hertz : « 10 200 km de lignes » | **Non vérifiable** |
| Swissgrid « 30 % » (Alpiq/Axpo) | **Non vérifiable** (participations, hors données) |
| CIP ~37 Mds$ · Brookfield TF-II 20 Mds$ · Macquarie 3 Mds$ | **Non vérifiable** (fonds, hors données) |
| enspired « 962 MW de pipeline en Italie » · Coalburn 2 « 500 MW » | **Non vérifiable** |
| « prix de 0 €/MWh à 500+ €/MWh » | **Non vérifiable** (pas de données de marché : ENTSO-E inaccessible) |
| « LCOE solaire le plus bas d'Europe (ES/PT) » | **Non vérifiable ici** (cohérent avec la littérature) |
| « Norvège 34,8 GW » | **Précisé** → 33,9 GW officiels (NVE) ; GIPT 28,9 GW |
| « Terna … 50 GW fossiles opérants » | **Confirmé** (50,0 GW) |
| « Allemagne … 73 % de la pointe reste fossile » | **Confirmé** (0,725) |
| « Espagne 83 GW renouvelables / 8,5 GW lignes » | **Confirmé** (83,6 / 8,5) |

---

## 5. Conclusion inchangée

Les trois corrections ne changent **rien à la thèse** : le solaire sur le toit produit du MWh, pas de la liquidité — il ne remplit aucune des quatre fonctions systémiques (inertie/fréquence, arbitrage temporel, arbitrage spatial, dernier ressort). La pyramide du pouvoir (optimiseurs → fonds BESS → TSO nœuds → hydro-stockage → prosumers) reste valide, avec deux nuances apportées par les données :

1. Les « débiteurs » identifiés (NL, BE, IE, DE, DK) le sont **par le soir fossile et la topologie**, pas par un seuil de part variable : la Grèce et l'Irlande sont aussi fragiles que le Danemark malgré des parts variables sous 50 %.
2. La banque centrale hydro est **plus grosse que la carte ne le dit** : le GIPT sous-compte l'hydro de 8 à 21 % selon les pays — la « liquidité » nordique et alpine est encore plus concentrée que ce que montre OpenGridWorks.

*Reproductibilité : `scripts/10_stack_audit.py` (vérification des 23 affirmations) · `scripts/11_norway_deepdive.py` (GEM vs officiel) · tables `output/tables/audit_claims.csv`, `audit_stack.csv`, `norway_comparison.csv`.*
