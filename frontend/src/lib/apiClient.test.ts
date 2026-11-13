import assert from "node:assert/strict";
import { createServer, type Server } from "node:http";
import { after, before, test } from "node:test";

import { ApiError, NetworkError, api, apiRequest, getSessionToken, setSessionToken, setUnauthorizedHandler } from "./apiClient";

let server: Server;
let baseUrl: string;
const seen: Array<{ path: string; method: string; auth?: string; body: string }> = [];

before(async () => {
  server = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      seen.push({
        path: request.url ?? "",
        method: request.method ?? "",
        auth: request.headers.authorization,
        body,
      });

      const send = (status: number, payload: string, contentType = "application/json") => {
        response.writeHead(status, { "Content-Type": contentType });
        response.end(payload);
      };

      if (request.url?.startsWith("/api/v1/echo")) {
        send(200, JSON.stringify({ method: request.method, query: request.url.split("?")[1] ?? "" }));
      } else if (request.url?.startsWith("/api/v1/boom")) {
        send(500, JSON.stringify({ error: "internal detail that must not leak" }));
      } else if (request.url?.startsWith("/api/v1/malformed")) {
        send(200, "not json at all");
      } else if (request.url?.startsWith("/api/v1/empty")) {
        send(204, "");
      } else if (request.url?.startsWith("/api/v1/unauthorized")) {
        send(401, JSON.stringify({ error: "authentication required" }));
      } else if (request.url?.startsWith("/api/v1/flaky")) {
        // 401 until the caller presents a refreshed token, then 200. Used to
        // prove the replay-after-refresh path.
        if (request.headers.authorization === "Bearer good-token") {
          send(200, JSON.stringify({ ok: true }));
        } else {
          send(401, JSON.stringify({ error: "token expired" }));
        }
      } else {
        send(404, JSON.stringify({ error: "not found" }));
      }
    });
  });

  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (address === null || typeof address === "string") {
    throw new Error("server did not bind to a port");
  }
  baseUrl = `http://127.0.0.1:${address.port}/api/v1`;
  process.env.NEXT_PUBLIC_API_URL = baseUrl;
});

after(async () => {
  await new Promise<void>((resolve) => server.close(() => resolve()));
});

test("get attaches no authorization header when no token is set", async () => {
  setSessionToken(null);
  await api.get("/echo");
  const last = seen[seen.length - 1];
  assert.equal(last.auth, undefined);
});

test("get attaches the bearer token once a session exists", async () => {
  setSessionToken("test-token");
  assert.equal(getSessionToken(), "test-token");
  await api.get("/echo");
  assert.equal(seen[seen.length - 1].auth, "Bearer test-token");
  setSessionToken(null);
});

test("authenticated:false omits the bearer token", async () => {
  setSessionToken("test-token");
  await api.post("/echo", { a: 1 }, { authenticated: false });
  const last = seen[seen.length - 1];
  assert.equal(last.auth, undefined);
  assert.equal(last.body, JSON.stringify({ a: 1 }));
  setSessionToken(null);
});

test("query parameters are encoded into the URL", async () => {
  const result = (await api.get("/echo", { query: { symbol: "BTC/RUSD", page: 2, skip: undefined } })) as {
    query: string;
  };
  assert.match(result.query, /symbol=BTC%2FRUSD/);
  assert.match(result.query, /page=2/);
  assert.ok(!result.query.includes("skip"));
});

test("a 5xx response becomes an ApiError without leaking server detail", async () => {
  await assert.rejects(
    () => api.get("/boom"),
    (error: unknown) => {
      assert.ok(error instanceof ApiError);
      assert.equal((error as ApiError).status, 500);
      assert.ok(!(error as ApiError).message.includes("internal detail"));
      return true;
    },
  );
});

test("a malformed JSON body becomes an ApiError", async () => {
  await assert.rejects(
    () => api.get("/malformed"),
    (error: unknown) => error instanceof ApiError && (error as ApiError).code === "invalid_json",
  );
});

test("an empty body resolves to null", async () => {
  assert.equal(await api.get("/empty"), null);
});

test("401 is surfaced as unauthorized", async () => {
  await assert.rejects(
    () => api.get("/unauthorized"),
    (error: unknown) => error instanceof ApiError && (error as ApiError).isUnauthorized,
  );
});

test("an unreachable host becomes a NetworkError", async () => {
  process.env.NEXT_PUBLIC_API_URL = "http://127.0.0.1:1/api/v1";
  await assert.rejects(() => apiRequest("/echo"), NetworkError);
  process.env.NEXT_PUBLIC_API_URL = baseUrl;
});

test("a request that exceeds its timeout becomes a NetworkError", async () => {
  await assert.rejects(
    () => apiRequest("/echo", { timeoutMs: 1 }),
    NetworkError,
  );
});

test("a 401 triggers the unauthorized handler and replays the request once", async () => {
  setSessionToken("expired-token");
  let handlerCalls = 0;
  let recover = false;
  setUnauthorizedHandler(async () => {
    handlerCalls += 1;
    return recover;
  });

  // With the handler declining, the 401 is surfaced and the handler ran once.
  await assert.rejects(() => api.get("/unauthorized"), (e: unknown) => e instanceof ApiError);
  assert.equal(handlerCalls, 1);

  // With the handler recovering, the request is replayed with the new token and
  // succeeds.
  recover = true;
  const handler = async () => {
    handlerCalls += 1;
    setSessionToken("good-token");
    return true;
  };
  setUnauthorizedHandler(handler);
  const body = await api.get("/flaky");
  assert.deepEqual(body, { ok: true });
  // The handler ran once for the retry; total calls across both phases is 2.
  assert.equal(handlerCalls, 2);

  setUnauthorizedHandler(null);
  setSessionToken(null);
});

test("the refresh call itself is never retried, so a failing refresh cannot recurse", async () => {
  let handlerCalls = 0;
  setUnauthorizedHandler(async () => {
    handlerCalls += 1;
    return true; // always claim success
  });

  // A 401 on an unauthenticated request must not invoke the handler at all.
  await assert.rejects(
    () => api.post("/unauthorized", undefined, { authenticated: false }),
    (e: unknown) => e instanceof ApiError,
  );
  assert.equal(handlerCalls, 0);

  setUnauthorizedHandler(null);
});
