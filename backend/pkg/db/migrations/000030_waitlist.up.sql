CREATE TABLE IF NOT EXISTS waitlist_signups (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT        NOT NULL UNIQUE,
    what_they_write TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
