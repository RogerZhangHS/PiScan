
-- These tables comprise the local datastore on the Raspberry Pi client
-- device, using SQLite for the database. SQLite has a limited set of
-- datatypes (https://www.sqlite.org/datatype3.html), so the analogous
-- server database columns have been adjusted accordingly.

-- `account` defines basic end-user information, corresponding to the
-- account table in the server database

CREATE TABLE IF NOT EXISTS student (
  name text NOT NULL,
  stuid integer PRIMARY KEY,
  submission_status boolean DEFAULT 0,
  submission_time integer DEFAULT 0,
  UNIQUE(name, stuid)
);
