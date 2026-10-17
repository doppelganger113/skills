# JavaScript and TypeScript (Node.js and browser)

Contents
- Static checks
- Async and promises
- Runtime types and values
- Event loop and resources
- Security
- Scheduled jobs and browser automation

## Static checks

Most JavaScript tooling runs from `node_modules` and loads configuration written in JavaScript, so most checks here are class E. Never run `npm install`, `npm ci`, `yarn`, `pnpm install`, or `npx` without `--no-install`; they download and run install scripts.

| Command | Class | Notes |
|---|---|---|
| `tsc --noEmit -p <tsconfig>` | S if `tsc` is installed outside the repository; E if it resolves to `node_modules/.bin` | Type-checks without emitting. |
| `biome lint` | S if Biome is installed outside the repository (its config is JSON) | |
| `eslint .` | E | Flat config (`eslint.config.*`) is JavaScript, and plugins run from `node_modules`. |
| package scripts (`npm run lint`, `npm test`) | E | They run whatever `package.json` defines. |
| `npm audit --omit=dev` | N | Needs a lockfile; runs no scripts. |
| `prettier --write`, codemods | Never | They write files. |

## Async and promises

- A promise that is neither awaited nor handled (a floating promise): errors vanish, or become an unhandled rejection, which ends the process by default in current Node.js.
- `array.forEach(async ...)`: nothing waits for the callbacks, so the function returns before the work is done.
- `Promise.all` over a user-sized list: unbounded concurrency against a database or API; and when one rejects, the rest keep running, uncancelled.
- Express 4 doesn't catch rejections from `async` route handlers: the request hangs and the error goes to the process. Express 5 does catch them. Check the version.
- Streams combined with `.pipe()` without error handling on every stream; `stream.pipeline` handles errors and cleanup.
- Retries without backoff, or around non-idempotent calls.

## Runtime types and values

- TypeScript types are not runtime checks: data from requests, files, queues and other services typed with `as` or `any` and never validated at the boundary.
- Numbers past 2^53 lose precision: 64-bit IDs from JSON or databases. `node-postgres` returns `bigint` (`int8`) columns as strings by default, so code may concatenate where it means to add.
- `process.env` values are strings: `"false"` and `"0"` are truthy.
- `Array.prototype.sort()` without a comparator sorts as strings, so `[10, 9, 1]` becomes `[1, 10, 9]`. It also sorts in place, mutating shared arrays.
- `Date`: months are 0-based; `new Date("2026-10-17")` is midnight UTC while `new Date("2026-10-17T00:00")` is local time; arithmetic across DST changes.
- Floating point for money.
- Loose equality (`==`) and truthiness checks that treat `0` or `""` as missing.

## Event loop and resources

- Synchronous work in a request path (`fs.*Sync`, `crypto.*Sync`, large `JSON.parse`, CPU-heavy loops) blocks every other request.
- Regular expressions with nested quantifiers on user input (catastrophic backtracking).
- `fetch` has no default timeout; pass `AbortSignal.timeout(...)`.
- Module-level caches or maps that only grow; event listeners added per request and never removed.
- Database or HTTP clients created per request instead of once, exhausting connections or sockets.

## Security

- Merging user-supplied JSON into objects (deep merge, `Object.assign` from parsed input): prototype pollution through `__proto__` or `constructor`.
- `path.join` or `path.resolve` with user input: traversal, and an absolute second argument to `resolve` discards the base.
- Server-side `fetch` of a user-supplied URL (SSRF), `child_process.exec` with interpolated input, template rendering without escaping.
- Secrets or server-only config reaching a client bundle, for example through public-prefixed environment variables.

## Scheduled jobs and browser automation

- A scheduled job that can still be running when the next run starts, with no lock: overlapping runs double the work or interleave writes.
- Playwright or Puppeteer: the browser or context not closed in `finally`, so failures leak processes and memory; fixed sleeps (`waitForTimeout`) instead of waiting for a condition; no overall timeout per run; an `expect` that isn't awaited, so it never fails the run.
- Scrapers that treat a missing element or a changed page as "no data" instead of an error, silently writing empty results.
