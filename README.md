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

```bash
go test -v -cover ./...
```

Les tests utilisent une implémentation **en mémoire** de l'interface `Store`
(`mem_store_test.go`), ce qui permet d'exécuter la logique métier et l'API (`httptest`)
sans base de données. Ils sont *table-driven* et couvrent les principaux cas métier
(crédits de bienvenue, crédits insuffisants, conflit de réservation, cycle de vie
complet, règles d'évaluation).

## Architecture

Un seul package Go, mais trois responsabilités clairement séparées :

| Couche       | Fichiers                                        | Rôle                                                        |
| ------------ | ----------------------------------------------- | ----------------------------------------------------------- |
| **HTTP**     | `handlers.go`, `response.go`, `middleware.go`   | Décodage/encodage JSON, routage, middlewares. *Aucune règle métier.* |
| **Métier**   | `app.go`, `models.go`, `errors.go`              | Validations, crédits, cycle de vie des échanges.            |
| **Stockage** | `store.go`, `sqlstore*.go`, `schema.go`         | Accès `database/sql` derrière l'interface `Store`.          |

La couche métier (`App`) dépend de l'interface `Store`, définie **côté consommateur**.
En production elle est satisfaite par `SQLStore` (PostgreSQL) ; en test par `memStore`.

Points notables :

- **Gestion d'erreurs idiomatique** : erreurs sentinelles (`ErrValidation`, `ErrNotFound`,
  `ErrConflict`…), enveloppées avec `%w` et discriminées via `errors.Is` pour produire le
  bon code HTTP (`errors.go`).
- **Concurrence gérée par la base** : aucun mutex. L'unicité « un seul échange actif par
  service » est garantie par un index unique partiel PostgreSQL ; les mouvements de
  crédits et les transitions d'état se font dans des transactions SQL.
- **`context`** : propagé à toutes les requêtes SQL, avec un timeout par requête HTTP et un
  arrêt gracieux du serveur (`signal.NotifyContext` + `server.Shutdown`).
