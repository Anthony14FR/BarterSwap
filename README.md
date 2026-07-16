# BarterSwap — API d'échange de compétences

Banque de temps entre particuliers : chaque heure de service rendue donne droit à une
heure de service reçue. La monnaie d'échange est le **crédit-temps**, jamais l'argent.

API REST écrite **uniquement avec la bibliothèque standard** de Go (`net/http`,
`encoding/json`, `database/sql`, `context`). Seule dépendance externe : le pilote
PostgreSQL `github.com/lib/pq`.

## Installation

```bash
git clone <url>
cd BarterSwap
go mod tidy
```

La base est créée automatiquement au démarrage (les tables sont migrées via
`CREATE TABLE IF NOT EXISTS`). Il suffit d'une base PostgreSQL accessible.

### Configuration (variables d'environnement)

| Variable       | Défaut                                                                     | Rôle                     |
| -------------- | -------------------------------------------------------------------------- | ------------------------ |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/barterswap?sslmode=disable`   | Chaîne de connexion      |
| `PORT`         | `8080`                                                                      | Port d'écoute HTTP       |

```bash
# Exemple : lancer une base jetable avec Docker
docker run --name barterswap-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=barterswap -p 5432:5432 -d postgres

go run .
```

## Authentification

Aucune authentification avancée : le client s'identifie via le header **`X-UserID`**
(l'identifiant numérique de l'utilisateur). Les routes de lecture publiques n'en ont pas
besoin ; les routes de création/modification l'exigent (sinon `401`).

## Endpoints

| Méthode  | Path                               | Auth | Description                                    |
| -------- | ---------------------------------- | :--: | ---------------------------------------------- |
| `POST`   | `/api/users`                       |      | Créer un compte (10 crédits de bienvenue)      |
| `GET`    | `/api/users/{id}`                  |      | Profil public                                  |
| `PUT`    | `/api/users/{id}`                  |  ✓   | Modifier son profil                            |
| `GET`    | `/api/users/{id}/skills`           |      | Compétences d'un utilisateur                   |
| `PUT`    | `/api/users/{id}/skills`           |  ✓   | Définir ses compétences (écrasement complet)   |
| `GET`    | `/api/users/{id}/reviews`          |      | Avis reçus                                      |
| `GET`    | `/api/users/{id}/stats`            |      | Statistiques                                    |
| `GET`    | `/api/services`                    |      | Lister les services (`?categorie=`, `?ville=`, `?search=`) |
| `POST`   | `/api/services`                    |  ✓   | Publier une annonce                            |
| `GET`    | `/api/services/{id}`               |      | Détail d'un service                            |
| `PUT`    | `/api/services/{id}`               |  ✓   | Modifier son annonce                           |
| `DELETE` | `/api/services/{id}`               |  ✓   | Supprimer son annonce                          |
| `GET`    | `/api/services/{id}/reviews`       |      | Avis sur un service                            |
| `POST`   | `/api/exchanges`                   |  ✓   | Créer une demande d'échange                    |
| `GET`    | `/api/exchanges`                   |  ✓   | Mes échanges (`?status=`)                       |
| `GET`    | `/api/exchanges/{id}`              |  ✓   | Détail d'un échange (participants seulement)    |
| `PUT`    | `/api/exchanges/{id}/accept`       |  ✓   | Accepter (offreur) — bloque les crédits         |
| `PUT`    | `/api/exchanges/{id}/reject`       |  ✓   | Refuser (offreur)                               |
| `PUT`    | `/api/exchanges/{id}/complete`     |  ✓   | Terminer — transfère les crédits                |
| `PUT`    | `/api/exchanges/{id}/cancel`       |  ✓   | Annuler — restitue les crédits bloqués          |
| `POST`   | `/api/exchanges/{id}/review`       |  ✓   | Noter un échange terminé                        |

### Compétences et catégories

Un service ne peut être publié que dans une catégorie pour laquelle le fournisseur a
déclaré une compétence : `Service.Categorie` doit correspondre (insensible à la casse) au
`Skill.Nom` d'une des compétences du profil (`PUT /api/users/{id}/skills`). Sinon, `400`.

### Cycle de vie d'un échange

```
pending ──accept──▶ accepted ──complete──▶ completed
   │                   │
 reject/cancel       cancel
   ▼                   ▼
rejected           cancelled
```

- **accepted** : les crédits sont *bloqués* (débités du demandeur, pas encore crédités à l'offreur).
- **completed** : les crédits sont *transférés* définitivement à l'offreur.
- **cancelled / rejected** : les crédits bloqués sont *restitués* au demandeur.

Le solde de chaque utilisateur est calculé à partir d'un **journal de transactions**
(`credit_transactions`), pas d'un simple compteur, ce qui garantit la traçabilité.

## Exemples d'utilisation

```bash
# 1. Créer deux utilisateurs (10 crédits de bienvenue chacun)
curl -s -X POST localhost:8080/api/users \
  -d '{"pseudo":"Tom","ville":"Paris"}'
curl -s -X POST localhost:8080/api/users \
  -d '{"pseudo":"Thami","ville":"Marseille"}'

# 2. Tom (id 1) déclare ses compétences puis publie un service
curl -s -X PUT localhost:8080/api/users/1/skills -H 'X-UserID: 1' \
  -d '[{"nom":"Jardinage","niveau":"expert"}]'
curl -s -X POST localhost:8080/api/services -H 'X-UserID: 1' \
  -d '{"titre":"Tonte de pelouse","categorie":"Jardinage","duree_minutes":60,"credits":4,"ville":"Paris"}'

# 3. Thami (id 2) demande l'échange sur le service 1
curl -s -X POST localhost:8080/api/exchanges -H 'X-UserID: 2' \
  -d '{"service_id":1}'

# 4. Tom accepte (crédits bloqués), puis on termine (crédits transférés)
curl -s -X PUT localhost:8080/api/exchanges/1/accept   -H 'X-UserID: 1'
curl -s -X PUT localhost:8080/api/exchanges/1/complete -H 'X-UserID: 1'

# 5. Thami note l'échange terminé, puis on consulte les stats de Tom
curl -s -X POST localhost:8080/api/exchanges/1/review -H 'X-UserID: 2' \
  -d '{"note":5,"commentaire":"Impeccable"}'
curl -s localhost:8080/api/users/1/stats
```

## Tests

La suite a deux modes. **Le mode complet est celui qui compte** : sans base, les tests
d'intégration se skippent et la couverture tombe sous le seuil.

```bash
# 1. Une base de test jetable
docker run --rm -d --name barterswap-test-db \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=barterswap_test \
  -p 55432:5432 postgres:17-alpine

# 2. La suite complète
export TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55432/barterswap_test?sslmode=disable"
go test -v -cover ./...      # → coverage: 71.5% of statements
```

```bash
# Mode rapide, sans base : les tests d'intégration se skippent
go test -v -cover ./...      # → coverage: 43.2% of statements
```

| Mode                        | Couverture | Ce qui est exercé                                    |
| --------------------------- | ---------- | ---------------------------------------------------- |
| `TEST_DATABASE_URL` défini  | **71,5 %** | Tout, `sqlstore*.go` compris                          |
| Sans base                   | 43,2 %     | Métier + API seulement ; `sqlstore*.go` non couvert   |

L'écart s'explique en une ligne : `sqlstore.go` et `sqlstore_exchange.go` pèsent ~43 %
des instructions et ne peuvent être exercés que contre un vrai PostgreSQL (transactions,
index unique partiel, codes d'erreur `pq`). Les mocker n'aurait rien prouvé.

Les tests unitaires utilisent une implémentation **en mémoire** de `Store`
(`store_mem_test.go`) : la logique métier et l'API (`httptest`) tournent sans base. Ils
sont *table-driven* et couvrent les cas métier du sujet — crédits de bienvenue, crédits
insuffisants, conflit de réservation, cycle de vie complet (accept / complete / reject /
cancel), règles d'évaluation, cohérence des stats.

## Architecture

Le sujet impose **un seul package Go, sans sous-packages**. En Go un dossier *est* un
package : « un seul package » implique donc mécaniquement des fichiers à plat. Les
couches sont rendues visibles par un **préfixe de nom** — l'ordre alphabétique devient
l'ordre des couches. C'est l'idiome de `crypto/tls` (`handshake_client.go`,
`handshake_server.go`…).

```
main.go                        point d'entrée : pool, migration, serveur, arrêt gracieux

biz_app.go                     métier — App, règles de gestion, cycle de vie
biz_errors.go                  métier — sentinelles, type Error
biz_models.go                  métier — types du domaine

http_middleware.go             exposition — recovery, logging, CORS, auth, timeout
http_response.go               exposition — encodage/décodage JSON, statusFor
http_server.go                 exposition — routes et handlers

store.go                       contrat — les 4 interfaces composées en Store
store_postgres.go              stockage — users, skills, services
store_postgres_exchange.go     stockage — échanges, crédits, avis
store_schema.go                stockage — DDL
```

| Couche       | Fichiers        | Rôle                                                                 |
| ------------ | --------------- | -------------------------------------------------------------------- |
| **Métier**   | `biz_*.go`      | Validations, crédits, cycle de vie. *Ne connaît ni HTTP ni SQL.*      |
| **HTTP**     | `http_*.go`     | Décodage/encodage JSON, routage, middlewares. *Aucune règle métier.*  |
| **Stockage** | `store*.go`     | Accès `database/sql` derrière l'interface `Store`.                    |

La séparation n'est pas qu'une affirmation : elle se vérifie en deux commandes.

```bash
# La couche métier n'importe ni HTTP ni SQL → aucun résultat
grep -lE '^\s+"(net/http|database/sql)"' biz_*.go

# Les autres couches, elles, les importent bien → la commande discrimine
grep -lE '^\s+"(net/http|database/sql)"' http_*.go store_*.go
```

Les fichiers `biz_*` n'utilisent que `context`, `fmt`, `strings` et `errors`. La
traduction des erreurs métier en codes HTTP est isolée dans `statusFor`
(`http_response.go`) : `biz_errors.go` produit des sentinelles sans jamais savoir
qu'elles deviendront un `400` ou un `409`.

La couche métier (`App`) dépend de l'interface `Store`, définie **côté consommateur**.
En production elle est satisfaite par `SQLStore` (PostgreSQL) ; en test par `memStore`.
Aucune des deux ne nomme jamais `Store` : la satisfaction est implicite.

`Store` n'est pas une interface monolithique : c'est la **composition** de quatre
interfaces plus petites, une par domaine métier.

```go
type Store interface {
    UserStore      // comptes, compétences, statistiques
    ServiceStore   // annonces
    ExchangeStore  // échanges + journal de crédits
    ReviewStore    // avis
}
```

Points notables :

- **Gestion d'erreurs idiomatique** : erreurs sentinelles (`ErrValidation`, `ErrNotFound`,
  `ErrConflict`…), enveloppées avec `%w` et discriminées via `errors.Is` pour produire le
  bon code HTTP (`errors.go`).
- **Concurrence gérée par la base** : aucun mutex. L'unicité « un seul échange actif par
  service » est garantie par un index unique partiel PostgreSQL ; les mouvements de
  crédits et les transitions d'état se font dans des transactions SQL.
- **`context`** : propagé à toutes les requêtes SQL, avec un timeout par requête HTTP et un
  arrêt gracieux du serveur (`signal.NotifyContext` + `server.Shutdown`).
