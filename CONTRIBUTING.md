# Contributing to DocMind

Thanks for improving DocMind. Keep pull requests focused, include tests for changed behavior, and update both English and Russian documentation when user-visible behavior changes.

## Local checks

Run the standard suite before opening a pull request:

```bash
make test
make lint
```

PostgreSQL integration tests need a disposable pgvector database and an explicit connection string:

```bash
DOCMIND_TEST_DATABASE_URL='postgres://docmind:docmind@localhost:5432/docmind?sslmode=disable' \
  make test-integration
```

For a full local demo, run `make up`, verify `make health`, then stop it with `make down`.
