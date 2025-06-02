import pandas as pd
import numpy as np
import xgboost as xgb
import shap
import umap
import hdbscan
from sklearn.preprocessing import LabelEncoder
import matplotlib.pyplot as plt
import seaborn as sns

# 1. Dataset
options = [
    ['AI Gateway', 'AI', 'Medium', 'High', 'Medium', 'High', 'High', 'Yes', 'Yes', 5000],
    ['Healthcare Gateway', 'Healthcare', 'Very High', 'Very High', 'High', 'Medium', 'High', 'No', 'Yes', 8000],
    ['Fintech Gateway', 'Fintech', 'High', 'Very High', 'High', 'High', 'Medium', 'No', 'Yes', 10000],
    ['Observability Pivot', 'Horizontal', 'Low', 'Medium', 'Medium', 'High', 'Medium', 'Yes', 'Yes', 3000],
    ['Reverse Proxy (pure)', 'Generic', 'None', 'Very Low', 'Low', 'Very Low', 'Very Low', 'Yes', 'No', 100],
]
columns = ['Option', 'Niche', 'Compliance Req', 'Pricing Power', 'Complexity', 'TAM Estimate', 'Stickiness', 'Free Competitor', 'Business Value Clear', 'Expected Monthly Revenue']
df = pd.DataFrame(options, columns=columns)

# 2. Encodage
feature_cols = ['Niche', 'Compliance Req', 'Pricing Power', 'Complexity', 'TAM Estimate', 'Stickiness', 'Free Competitor', 'Business Value Clear']
X = df[feature_cols].copy()
ord_map = {'None': 0, 'Low': 1, 'Medium': 2, 'High': 3, 'Very High': 4, 'Very Low': 0}
for col in ['Compliance Req', 'Pricing Power', 'Complexity', 'TAM Estimate', 'Stickiness']:
    X[col] = X[col].map(ord_map)
X['Free Competitor'] = X['Free Competitor'].map({'Yes': 1, 'No': 0})
X['Business Value Clear'] = X['Business Value Clear'].map({'Yes': 1, 'No': 0})
X['Niche'] = LabelEncoder().fit_transform(X['Niche'])
y = df['Expected Monthly Revenue']

# 3. XGBoost
model = xgb.XGBRegressor(objective='reg:squarederror', n_estimators=50, random_state=42)
model.fit(X, y)
print('Score R2:', model.score(X, y))

# 4. SHAP
explainer = shap.Explainer(model)
shap_values = explainer(X)
shap.summary_plot(shap_values, X, plot_type='bar', show=False)
plt.tight_layout()
plt.savefig('shap_importance.png')
print('SHAP importance plot saved as shap_importance.png')

# 5. UMAP + HDBSCAN
reducer = umap.UMAP(random_state=42)
embedding = reducer.fit_transform(X)
clusterer = hdbscan.HDBSCAN(min_cluster_size=2)
labels = clusterer.fit_predict(embedding)
df['Cluster'] = labels
plt.figure(figsize=(8,6))
sns.scatterplot(x=embedding[:,0], y=embedding[:,1], hue=labels, palette='tab10', s=100)
plt.title('Clusters de stratégies (UMAP + HDBSCAN)')
plt.tight_layout()
plt.savefig('clusters.png')
print('Cluster plot saved as clusters.png')

print('\nRésultats des clusters:')
print(df[['Option', 'Cluster', 'Expected Monthly Revenue']]) 