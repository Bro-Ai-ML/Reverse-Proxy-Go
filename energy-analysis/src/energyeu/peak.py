"""Pointes de demande hiver approximatives par pays (GW).

Ordres de grandeur ENTSO-E ~2024 (pointe hiver / forte pointe). Sources :
ENTSO-E Transparency Platform, rapports des gestionnaires de réseau nationaux.
Valeurs indicatives utilisées uniquement pour des normalisations relatives.
"""

PEAK_DEMAND_GW: dict[str, float] = {
    "DEU": 85.0, "FRA": 90.0, "GBR": 45.0, "ITA": 60.0, "ESP": 45.0,
    "POL": 28.0, "SWE": 27.0, "NOR": 25.0, "NLD": 18.0, "FIN": 14.0,
    "BEL": 14.0, "AUT": 11.0, "CZE": 11.0, "PRT": 11.0, "ROU": 9.0,
    "GRC": 10.0, "CHE": 9.0, "HUN": 7.0, "DNK": 6.5, "BGR": 6.5,
    "IRL": 6.5, "SRB": 7.0, "HRV": 3.5, "SVK": 5.0, "LTU": 3.0,
    "LVA": 2.0, "EST": 1.6, "SVN": 3.0, "BIH": 2.5, "ALB": 1.8,
    "MKD": 1.6, "MNE": 0.6, "XKX": 1.2, "UKR": 23.0, "MDA": 1.6,
    "TUR": 55.0, "GEO": 2.2, "BLR": 6.0, "ISL": 2.4, "CYP": 1.4,
    "MLT": 0.5, "ARM": 1.5, "AZE": 6.0, "LUX": 1.0,
}
