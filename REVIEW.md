# 🔬 Code Review — Reverse-Proxy-Go

> Audit complet de la codebase (`go-backend/`, `infra/`, CI, Docker) — 2026-08-22.
> Chaque issue est localisée (fichier), classée par sévérité, avec la correction apportée dans cette branche.

**Sévérités** : 🔴 Critique (crash, sécurité, perte d'argent/données) · 🟠 Majeur (bug fonctionnel, race) · 🟡 Mineur (robustesse, hygiène) · 🔵 Info/dette.

---

## 1. Verdict global & scores

| Axe | Score /10 | Résumé |
|---|---|---|
| Architecture & conception | 5.0 | Bon découpage microservices + packages partagés, mais le "reverse proxy" n'existe pas, frontières de services floues, 3 schémas de noms de modules. |
| Correction du code | 2.5 | ≥ 4 services ne **compilent pas** ; panique nil garantie sur `/payments` ; paniques sur type assertions ; data races. |
| Sécurité | 2.0 | RBAC par header client falsifiable, credentials hardcodés, endpoints de paiement publics, spoofing X-Forwarded-For, pas d'idempotency keys Stripe. |
| Fiabilité / intégrité des données | 2.5 | Usage de facturation **perdu silencieusement** en cas d'échec Stripe ; queues en mémoire at-most-once ; shutdown sans drain. |
| Tests | 4.0 | Vrais unit tests sur shared/ et payment/, mais un test contredit son impl (validate), la CI ne compile rien, `-race` absent. |
| DevOps / CI / Docker | 2.0 | CI cassée dès l'étape `go mod tidy` ; binaires 20 Mo commités ; compose qui référence des Dockerfiles inexistants. |
| Hygiène repo | 3.0 | Binaires, coverage `.out`, template `your-app` non renommé, Dockerfile Python orphelin, README qui décrit des services inexistants. |

### **Note globale : 2.9 / 10** — prototype cassé, impropre à la prod.
La bonne nouvelle : les fondations (chi/mux, slog, packages partagés, tests partiels) sont saines. Le plan de correction ci-dessous est implémenté dans cette branche.

---

## 2. Issues critiques (🔴)

### C-1. `payment-service` : panique nil sur CHAQUE requête `/api/v1/payments`
`main.go` : `handlers.NewService(nil, stripeClient)` — `cfg` est `nil`, puis `CreatePayment`/`GetPayment` déréférencent `s.config.RequestTimeout` → **runtime panic, le process meurt** (mux n'a pas de recoverer). Aucun paiement ne peut passer, et chaque tentative crash le service.
✅ Corrigé : `config.DefaultConfig()` + `Validate()`, `cfg` passé au service.

### C-2. `auth-service` et `billing-service` : ne compilent pas
- `auth-service/cmd/server/main.go:66` route vers `h.VerifyToken` — méthode inexistante ; `handler.go` utilise `validate`, `context`, `time`, `jwt` jamais importés/déclarés.
- `billing-service/cmd/server/main.go` enregistre ~15 handlers (`ListInvoices`, `PayInvoice`, `AuthMiddleware`, …) dont seuls 2 existent.
Bug **silencieux** : la CI ne build pas ces modules (voir D-1), donc personne ne le voit.
✅ Corrigé : imports restaurés, validation hand-rolled (sans dépendance), `VerifyToken` implémenté ; stubs 501 + `AuthMiddleware` HS256 fail-closed pour billing.

### C-3. Graphe de modules cassé (`stripe-demo` vs `stripe-ecosystem` vs `shared/*`)
`payment-service`, `usage-service`, `customer-service`, `webhook-service` importent un middleware partagé, mais :
- leurs `go.mod` n'avaient aucun `require` pour ce module ;
- `shared/middleware/go.mod` déclarait `module shared/middleware` (le chemin importé ne pouvait donc **jamais** résoudre) ;
- `go.work` n'inclut ni `shared/contracts`, ni `shared/middleware`, ni `shared/logger` ;
- `usage-engine` n'avait **aucun go.mod** ; `template-service` s'appelait `your-app`.
→ Ces services ne compilent pas. Trois conventions de nommage coexistent (`stripe-demo`, `github.com/stripe-ecosystem/*`, `your-app`).
✅ Corrigé : module middleware renommé `github.com/stripe-ecosystem/shared/middleware` (aligné sur contracts/stripe-client), `require`+`replace` ajoutés dans les 4 services consommateurs, `go.work` complété, replaces morts du gateway supprimés, go.mod créé pour usage-engine, template-service renommé.

### C-4. RBAC falsifiable par le client
`shared/middleware/rbac.go` : le rôle est lu depuis le header **`X-User-Role` envoyé par le client**. N'importe qui peut envoyer `X-User-Role: admin` → escalade de privilèges triviale sur tout service qui l'utilise.
✅ Corrigé : `RBACMiddleware` lit désormais les rôles du **contexte** (posés par le middleware JWT) ; le fallback header est supprimé. Tests mis à jour.

### C-5. Gateway : credentials hardcodés + refresh token qui downgrade les rôles
- `handlers.go:validateCredentials` accepte `user`/`pass` en dur.
- `refresh_handler.go` : au refresh, les rôles sont remplacés par `[]string{"user"}` **en dur** → un admin qui refresh devient user (et surtout, les rôles ne sont jamais persistés avec le refresh token).
- `InMemoryRefreshTokenStore` : map sans **aucun mutex** → data race + crash possible (`fatal error: concurrent map writes`).
- Rotation sans détection de réutilisation ; token plaintext conservé en mémoire.
✅ Corrigé : validateur de credentials injectable (fail-closed, démo uniquement si env explicite), rôles stockés dans le refresh token, mutex, détection de reuse (révocation de toutes les sessions), lookup par hash via l'interface.

### C-6. Usage/facturation : perte silencieuse de données + totaux faux
- `batch/manager.go` : si `ProcessBatch` échoue (Stripe down, circuit ouvert), le batch est **jeté après un log** → sous-facturation silencieuse.
- `metered_billing.go` : `total_usage` est écrit avec le total **du batch courant**, pas un cumul → chaque batch écrase le précédent ; lecture/écriture concurrente = lost updates.
- L'"idempotency key" est générée en `UnixNano` et n'est **jamais envoyée à Stripe** (pas de `SetIdempotencyKey`) → les retries peuvent double-compter.
✅ Corrigé : requeue des batchs échoués (borné), cumul read-modify-write, vraie idempotency key passée à l'appel Stripe, `Stop()` idempotent, goroutines async trackées.

### C-7. Aucune authentification sur les APIs paiement / customers / usage
`payment-service`, `customer-service`, `usage-service` exposent leurs routes avec uniquement security headers + rate limit. Créer des PaymentIntents ou lire n'importe quel paiement par ID (IDOR) est ouvert à tous.
✅ Corrigé côté gateway (seul point d'entrée légitime) ; à terme chaque service doit vérifier un JWT de service — documenté dans REVIEW (les services écoutent sur réseau interne uniquement, compose sans ports publics pour les services internes).

### C-8. `usage-engine` : `KEYS usage:*` en prod + mémoire non bornée
`KEYS` est O(N) et bloque Redis ; les hashes n'ont **pas de TTL** ; `POST /event` accepte n'importe quel `customer_id` (DoS mémoire trivial).
✅ Corrigé : `SCAN`, TTL 8 jours sur chaque clé, sanitisation/longueur max du customer_id.

---

## 3. Issues majeures (🟠)

| # | Localisation | Problème | Fix |
|---|---|---|---|
| M-1 | `customer-service/main.go` | `http.Server{Addr: cfg.Port}` sans `:` → `ListenAndServe` échoue toujours ("missing port"). Le service ne démarre **jamais**. | `":" + cfg.Port` |
| M-2 | `payment-service/main.go` | Bind sur `:0` (port aléatoire) en ignorant `PORT` → le service est injoignable en déploiement. | Bind sur `cfg.Port` |
| M-3 | `gateway/main.go` | `privKey, _ := os.ReadFile(...)` : erreurs ignorées ; sans clés le gateway fail au démarrage avec un message cryptique. Type assertion `.(string)` sans virgule-ok → panique possible. | Fail-fast avec message clair, assertions sûres |
| M-4 | `gateway` | **Aucun reverse proxy** dans un repo nommé Reverse-Proxy-Go ; `healthcheck.go` (avec races) jamais branché. | Proxy `httputil.ReverseProxy` multi-upstreams + health-aware routing ajouté |
| M-5 | `healthcheck.go` | Écriture de `b.Healthy` dans des goroutines sans verrou (la map est verrouillée, les champs non) ; `resp.Body` jamais fermé (fuite de connexions) ; log d'erreur à chaque tick. | Mutex correct, `defer resp.Body.Close()`, log uniquement sur transition |
| M-6 | `shared/middleware/ratelimit.go` | `getIP` casse sur IPv6 (`Split(":")` sur `[::1]:80` → `"["`) ; XFF gauche toujours fiable → bypass du rate limit par spoofing. | `net.SplitHostPort`, option `TrustedProxies` (walk droite→gauche) |
| M-7 | `shared/middleware/circuitbreaker.go` | Utilise `httptest.NewRecorder()` **en production** : bufferise tout le body en mémoire (DoS/streaming), importe un paquet de test. | ResponseWriter streaming sans buffer ; 5xx comptés côté circuit, réponse transmise telle quelle |
| M-8 | `shared/middleware/jwt.go` | `log.Fatal` à l'init du package → tout import (y compris tests) tue le process si `JWT_SECRET` absent ; claims jamais propagés au contexte (auth sans identité). | Chargement lazy, fail-closed à la requête, claims dans le contexte |
| M-9 | `shared/middleware/validate.go` | Content-Type exact (rejette `application/json; charset=utf-8`) ; `ContentLength` falsifiable ; test existant contredit l'impl (GET sans body devrait passer). | `mime.ParseMediaType`, contrôle seulement si body présent, `MaxBytesReader` |
| M-10 | `auth-service` | Refresh tokens = JWT stateless **non révocables** (logout = mensonge) ; pas de rate limiting login ; pas de normalisation d'email. | Documenté + mitigation gateway ; `Validate()` renforcé |
| M-11 | `stripe-client/client.go` | Aucun `Idempotency-Key` sur `CreatePaymentIntent`/`CreateCustomer`/`CreateSubscription` → double charge possible au retry. | Idempotency keys générées sur tous les creates |
| M-12 | `user-service` | Pas de `main.go` (aucun binaire !) ; clés de contexte incohérentes (`"userID"` vs `"user_id"` vs clé typée) → les endpoints RGPD renvoyaient toujours 401. | `cmd/server/main.go` ajouté ; middleware auth pose `"user_id"` lu par les handlers RGPD |
| M-13 | `usage-service/worker/pool.go` | Shutdown = les events en queue sont jetés (at-most-once sur de la facturation). | Drain de la queue avant arrêt des workers |
| M-14 | `user_repository.go` | Tokens (email verify/reset) stockés **en clair** ; `AddUserRole`/`RemoveUserRole` read-modify-write sans transaction (lost update). | Hashage recommandé (TODO sécurisé) + roles en update atomique SQL |
| M-15 | `webhook-service` | Pas de déduplication d'events (l'objet actuel est log-only, mais la fondation est là) ; réponse 400 avec le détail d'erreur Stripe. | Message d'erreur générique |

---

## 4. Issues mineures & dette (🟡/🔵)

- 🟡 `gateway/main.go` : `middleware.Timeout(60s)` > `WriteTimeout(10s)` → comportement contradictoire.
- 🟡 `retry.go` : `rand.Seed(time.Now().UnixNano())` à chaque backoff (déprécié Go 1.20+, jitter corrélé sous concurrence) → remplacé par `rand.Float64()` global thread-safe (seed auto depuis 1.20).
- 🟡 `user-service/internal/middleware/rate_limiter.go` : fichier vide (TODO) alors que la doc promet du rate limiting.
- 🟡 `ioutil.ReadFile` encore utilisé (`gateway/config/secrets.go`) ; secret lu d'un fichier non trimé.
- 🟡 `customer-service` : `err.(validator.ValidationErrors)` sans garde → panique sur `InvalidValidationError`.
- 🟡 `fmt.Sscanf` sans vérifier l'erreur (`metered_billing.go`, `usage-engine`).
- 🟡 `httpserver/server.go` : `os.Exit(1)` en goroutine si le bind échoue ; pas de `BaseContext`.
- 🔵 `template-service` : module littéralement nommé `your-app` → renommé `template-service`.
- 🔵 Root `Dockerfile` : image Python/Jupyter orpheline du dossier `python-ml` supprimé, copie un `requirements.txt` inexistant → `docker build` échoue. Supprimé.
- 🔵 `README.md` : décrit `subscription-service`, `invoice-service`, ports inexactes. Mis à jour.
- 🔵 Binaires commités (`stripe-demo` 9,8 Mo, `webhook-service` 9,5 Mo, `gateway`, `customer-service`, `payment-service`, `usage-service`) + fichiers `coverage*.out`/`coverage.html` → retirés de git + `.gitignore`.
- 🔵 `infra/docker-compose.yml` : gateway hors réseau des backends ; compose build de services sans Dockerfile ; healthchecks `curl` sur images sans curl ; `jupyter-ml` mort. Corrigé.
- 🔵 `.github/workflows/ci.yml` : `go mod tidy` à la racine sans `go.mod` → échec immédiat ; golangci-lint installé depuis `master` via curl|sh. Réécrit (build+vet+test `-race` de tous les modules du workspace, versions épinglées).

---

## 5. La solution "god tier" implémentée dans cette branche

1. **Tout compile** : graphe de modules unifié, services réparés, CI qui build réellement le workspace (`go.work`) — la ceinture de sécurité qui manquait.
2. **Le gateway devient un vrai reverse proxy** : routing vers upstreams configurables par env, health-check actif (éjection des backends morts), timeouts sûrs, auth JWT RS256 avec issuer/audience validés, refresh tokens avec rôles + rotation + détection de reuse, aucun credential hardcodé (fail-closed).
3. **Argent protégé** : idempotency keys partout côté Stripe, batchs d'usage re-queueés en cas d'échec (plus de perte silencieuse), cumul d'usage correct.
4. **Sécurité par défaut** : RBAC par contexte (plus de header falsifiable), rate limiter IPv6-safe avec trusted proxies, circuit breaker streaming sans buffer, bodies limités (`MaxBytesReader`), secrets fail-closed.
5. **Hygiène** : binaires/artefacts hors de git, Dockerfiles alignés sur `go 1.24`, compose cohérent (réseaux, ports, fichiers existants), README fidèle à la réalité.

### Suites recommandées (hors périmètre de ce commit)
- Remplacer le store in-memory de refresh tokens par Redis/Postgres (multi-répliques).
- Persister les events d'usage (Postgres/Kafka) avant ACK — l'actuel at-most-once en mémoire reste un choix à assumer.
- Hasher les tokens email-verify/reset (M-14), brancher un vrai fournisseur d'identité pour les services internes (mTLS ou JWT service-to-service).
- Ajouter des tests d'intégration e2e gateway→upstream et des load tests sur le proxy.
