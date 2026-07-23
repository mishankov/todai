# Todai frontend

The frontend is a SvelteKit application. Bun 1.3.14 is its only JavaScript package manager and
runtime prerequisite.

```shell
bun install --frozen-lockfile
bun x playwright install chromium chromium-headless-shell
bun run dev
```

Run the complete local frontend check with:

```shell
bun run lint
bun run check
bun run test:unit --run
bun run build
bun run test:e2e
```

SvelteKit proxies `/api` to `TODAI_API_URL`, or to `http://localhost:8080` when it is unset.
