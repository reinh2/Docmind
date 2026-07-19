# DocMind widget asset

The browser-ready, single-file widget is embedded into the Go API at
`internal/httpapi/assets/docmind-widget.js` and served as `/docmind-widget.js`.

Keeping the runtime asset alongside the HTTP handler makes `docker compose up`
enough for a copy-paste install, with no Node build step or CDN dependency.
