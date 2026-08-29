"""Configuration : périmètres géographiques et mapping des noms de pays GEM."""

from __future__ import annotations

# ---------------------------------------------------------------------------
# Noms de pays tels qu'ils apparaissent dans le GIPT (colonne "Country/area")
# ---------------------------------------------------------------------------
GEM_NAME_TO_ISO3: dict[str, str] = {
    # Europe (région GEM)
    "Albania": "ALB",
    "Andorra": "AND",
    "Austria": "AUT",
    "Belarus": "BLR",
    "Belgium": "BEL",
    "Bosnia and Herzegovina": "BIH",
    "Bulgaria": "BGR",
    "Croatia": "HRV",
    "Czech Republic": "CZE",
    "Denmark": "DNK",
    "Estonia": "EST",
    "Faroe Islands": "FRO",
    "Finland": "FIN",
    "France": "FRA",
    "Germany": "DEU",
    "Gibraltar": "GIB",
    "Greece": "GRC",
    "Guernsey": "GGY",
    "Hungary": "HUN",
    "Iceland": "ISL",
    "Ireland": "IRL",
    "Isle of Man": "IMN",
    "Italy": "ITA",
    "Jersey": "JEY",
    "Kosovo": "XKX",
    "Latvia": "LVA",
    "Lithuania": "LTU",
    "Luxembourg": "LUX",
    "Malta": "MLT",
    "Moldova": "MDA",
    "Montenegro": "MNE",
    "Netherlands": "NLD",
    "North Macedonia": "MKD",
    "Norway": "NOR",
    "Poland": "POL",
    "Portugal": "PRT",
    "Romania": "ROU",
    "Russia": "RUS",
    "Serbia": "SRB",
    "Slovakia": "SVK",
    "Slovenia": "SVN",
    "Spain": "ESP",
    "Sweden": "SWE",
    "Switzerland": "CHE",
    "Ukraine": "UKR",
    "United Kingdom": "GBR",
    "Åland Islands": "ALA",
    # Asie occidentale / bassin méditerranéen / Caucase (région GEM "Asia")
    "Türkiye": "TUR",
    "Cyprus": "CYP",
    "Georgia": "GEO",
    "Armenia": "ARM",
    "Azerbaijan": "AZE",
    # Afrique du Nord
    "Morocco": "MAR",
    "Algeria": "DZA",
    "Tunisia": "TUN",
    "Libya": "LBY",
    "Egypt": "EGY",
    # Asie centrale (corridor "route de la soie énergétique")
    "Kazakhstan": "KAZ",
    "Kyrgyzstan": "KGZ",
    "Tajikistan": "TJK",
    "Turkmenistan": "TKM",
    "Uzbekistan": "UZB",
    "Mongolia": "MNG",
    # Moyen-Orient
    "Israel": "ISR",
    "Jordan": "JOR",
    "Lebanon": "LBN",
    "Syria": "SYR",
    "Saudi Arabia": "SAU",
    "Iraq": "IRQ",
    "Iran": "IRN",
    "United Arab Emirates": "ARE",
    "Oman": "OMN",
    "Qatar": "QAT",
    "Kuwait": "KWT",
    "Bahrain": "BHR",
    "Yemen": "YEM",
    # Chine (pour les corridors)
    "China": "CHN",
    "India": "IND",
    "Pakistan": "PAK",
    "Afghanistan": "AFG",
    # Afrique subsaharienne (corridors sud)
    "Nigeria": "NGA",
    "Senegal": "SEN",
    "Mauritania": "MRT",
    "Mali": "MLI",
    "Niger": "NER",
    "Chad": "TCD",
    "Sudan": "SDN",
    "Ethiopia": "ETH",
    "Kenya": "KEN",
    "Tanzania": "TZA",
    "Mozambique": "MOZ",
    "South Africa": "ZAF",
    "Congo, Democratic Republic of": "COD",
    "Democratic Republic of the Congo": "COD",
    "Cameroon": "CMR",
    "Ivory Coast": "CIV",
    "Côte d'Ivoire": "CIV",
    "Ghana": "GHA",
    "Angola": "AGO",
    "Zambia": "ZMB",
    "Zimbabwe": "ZWE",
    "Botswana": "BWA",
    "Namibia": "NAM",
    # Autres gros pays utiles
    "United States": "USA",
    "Canada": "CAN",
    "Australia": "AUS",
    "Japan": "JPN",
    "South Korea": "KOR",
    "Korea, Republic of": "KOR",
    "Indonesia": "IDN",
    "Vietnam": "VNM",
    "Thailand": "THA",
    "Malaysia": "MYS",
    "Brazil": "BRA",
    "Mexico": "MEX",
    "Argentina": "ARG",
    "Chile": "CHL",
    "Colombia": "COL",
}

# ---------------------------------------------------------------------------
# Ensembles de pays
# ---------------------------------------------------------------------------
EU27: set[str] = {
    "AUT", "BEL", "BGR", "HRV", "CYP", "CZE", "DNK", "EST", "FIN", "FRA",
    "DEU", "GRC", "HUN", "IRL", "ITA", "LVA", "LTU", "LUX", "MLT", "NLD",
    "POL", "PRT", "ROU", "SVK", "SVN", "ESP", "SWE",
}

# Union européenne + voisins synchrones/fortement connectés au réseau européen
EUROPE_GRID: set[str] = EU27 | {
    "GBR", "NOR", "CHE", "ALB", "BIH", "MKD", "MNE", "SRB", "XKX",
    "UKR", "MDA", "TUR", "GEO",
}

# Balkans occidentaux (hors UE)
WESTERN_BALKANS: set[str] = {"ALB", "BIH", "MKD", "MNE", "SRB", "XKX"}

# Europe large (région GEM Europe + Türkiye + Chypre + Caucase)
GREATER_EUROPE: set[str] = EUROPE_GRID | {"ARM", "AZE", "BLR", "ISL", "AND", "LIE", "MCO", "SMR", "VAT"}

# Bassin Sud (Afrique du Nord) — corridors sud
NORTH_AFRICA: set[str] = {"MAR", "DZA", "TUN", "LBY", "EGY"}

# Asie centrale — corridors est ("route de la soie énergétique")
CENTRAL_ASIA: set[str] = {"KAZ", "KGZ", "TJK", "TKM", "UZB"}

# Moyen-Orient / Golfe
MIDDLE_EAST: set[str] = {"ISR", "JOR", "LBN", "SYR", "SAU", "IRQ", "IRN",
                         "ARE", "OMN", "QAT", "KWT", "BHR", "YEM"}

# Corridor de la soie : Chine + Asie centrale + Caucase + Moyen-Orient
SILK_ROAD: set[str] = CENTRAL_ASIA | {"CHN", "MNG", "PAK", "AFG", "IRN"} | MIDDLE_EAST

# ---------------------------------------------------------------------------
# Regroupements de technologies pour les analyses de dispatchabilité
# ---------------------------------------------------------------------------
DISPATCHABLE_TYPES: set[str] = {
    "coal", "oil/gas", "nuclear", "bioenergy", "geothermal", "hydropower",
}
VARIABLE_TYPES: set[str] = {"wind", "utility-scale solar"}

FOSSIL_TYPES: set[str] = {"coal", "oil/gas"}

# Statuts considérés comme "actifs / en service"
ACTIVE_STATUSES: set[str] = {"operating"}

# Statuts pipeline (futur)
PIPELINE_STATUSES: set[str] = {"construction", "pre-construction", "announced"}

# Statuts échoués / zombies potentiels
STALLED_STATUSES: set[str] = {"announced", "pre-construction", "shelved"}

# Statuts morts
DEAD_STATUSES: set[str] = {"retired", "cancelled", "cancelled - inferred 4 y",
                           "shelved - inferred 2 y", "mothballed"}
