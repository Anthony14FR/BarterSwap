package main

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id         SERIAL PRIMARY KEY,
	pseudo     TEXT NOT NULL,
	bio        TEXT NOT NULL DEFAULT '',
	ville      TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skills (
	id      SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	nom     TEXT NOT NULL,
	niveau  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_skills_user ON skills(user_id);

CREATE TABLE IF NOT EXISTS services (
	id            SERIAL PRIMARY KEY,
	provider_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	titre         TEXT NOT NULL,
	description   TEXT NOT NULL DEFAULT '',
	categorie     TEXT NOT NULL,
	duree_minutes INTEGER NOT NULL,
	credits       INTEGER NOT NULL,
	ville         TEXT NOT NULL DEFAULT '',
	actif         BOOLEAN NOT NULL DEFAULT TRUE,
	created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_services_provider ON services(provider_id);

CREATE TABLE IF NOT EXISTS exchanges (
	id           SERIAL PRIMARY KEY,
	service_id   INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
	requester_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	owner_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	status       TEXT NOT NULL,
	created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_exchange_active
	ON exchanges(service_id) WHERE status IN ('pending', 'accepted');

CREATE TABLE IF NOT EXISTS credit_transactions (
	id          SERIAL PRIMARY KEY,
	user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	exchange_id INTEGER REFERENCES exchanges(id) ON DELETE SET NULL,
	montant     INTEGER NOT NULL,
	type        TEXT NOT NULL,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_txn_user ON credit_transactions(user_id);

CREATE TABLE IF NOT EXISTS reviews (
	id          SERIAL PRIMARY KEY,
	exchange_id INTEGER NOT NULL REFERENCES exchanges(id) ON DELETE CASCADE,
	author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	target_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	note        INTEGER NOT NULL,
	commentaire TEXT NOT NULL DEFAULT '',
	created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (exchange_id, author_id)
);
CREATE INDEX IF NOT EXISTS idx_reviews_target ON reviews(target_id);
`
