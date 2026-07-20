# Security policy

Please do not report security vulnerabilities in a public issue. Open a private security advisory for this repository instead, including a minimal reproduction and the potential impact.

DocMind's demo defaults are intentionally convenient for local evaluation. Before any internet-facing deployment, set a strong admin token, configure explicit CORS origins, keep secrets outside the repository, terminate TLS at the edge, and use a production-grade background-job system.
