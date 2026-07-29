CREATE TABLE IF NOT EXISTS customers (
  id integer PRIMARY KEY,
  name text NOT NULL,
  prefecture text NOT NULL,
  points integer NOT NULL
);
INSERT INTO customers VALUES
  (1, 'Alice', 'Tokyo', 120),
  (2, 'Bob', 'Osaka', 80),
  (3, 'Carol', 'Tokyo', 250),
  (4, 'Dave', 'Hokkaido', 60),
  (5, 'Eve', 'Osaka', 180)
ON CONFLICT (id) DO NOTHING;
