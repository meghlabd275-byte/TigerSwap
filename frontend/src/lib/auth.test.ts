import assert from "node:assert/strict";
import { createServer, type Server } from "node:http";
import { after, before, test } from "node:test";

import { getCurrentUser, login, logout, register, validatePassword, validateUsername } from "./auth";
import { setSessionToken } from "./apiClient";

let server: Server;
let baseUrl: string;
const requests: Array<{ path: string; body: Record<string, unknown> }> = [];

before(async () => {
  server = createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      const raw = Buffer.concat(chunks).toString("utf8");
      const path = request.url ?? "";
      let body: Record<string, unknown> = {};
      if (raw.length > 0) {
        body = JSON.parse(raw) as Record<string, unknown>;
      }
      requests.push({ path, body });

      const send = (status: number, payload: unknown) => {
        response.writeHead(status, { "Content-Type": "application/json" });
        response.end(JSON.stringify(payload));
      };

      if (path.endsWith("/auth/register")) {
        // Mirrors the server contract: username is required.
        if (typeof body.username !== "string" || body.username === "") {
          send(400, { error: "Key: 'Username' Error:Field validation for 'Username' failed on the 'required' tag" });
          return;
        }
        send(201, { token: "register-token", user: { id: "u1", email: body.email, role: "user" } });
      } else if (path.endsWith("/auth/login")) {
        send(200, { token: "login-token", user: { id: "u1", email: body.email, role: "user" } });
      } else if (path.endsWith("/auth/logout")) {
        send(200, { ok: true });
      } else if (path.endsWith("/user/profile")) {
        send(200, { id: "u1", email: "trader@tigerswap.local", role: "user" });
      } else {
        send(404, { error: "not found" });
      }
    });
  });

  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (address === null || typeof address === "string") {
    throw new Error("server did not bind");
  }
  baseUrl = `http://127.0.0.1:${address.port}/api/v1`;
  process.env.NEXT_PUBLIC_API_URL = baseUrl;
});

after(async () => {
  await new Promise<void>((resolve) => server.close(() => resolve()));
});

test("validateUsername mirrors the server rules", () => {
  assert.equal(validateUsername("tiger_trader-01.v2"), null);
  assert.ok(validateUsername("ab"));
  assert.ok(validateUsername("a".repeat(33)));
  assert.ok(validateUsername("<script>alert(1)</script>"));
  assert.ok(validateUsername("user name"));
});

test("validatePassword enforces the bcrypt byte limit", () => {
  assert.equal(validatePassword("Str0ng-Passw0rd!2026"), null);
  assert.ok(validatePassword("a".repeat(73)));
  assert.ok(validatePassword("short1A"));
});

// Before the fix, register() sent only email and password, so the server
// rejected every attempt with a 400 for the missing username.
test("register sends the username the server requires", async () => {
  requests.length = 0;
  await register("trader@tigerswap.local", "Str0ng-Passw0rd!2026", "tiger_trader");

  const registration = requests.find((entry) => entry.path.endsWith("/auth/register"));
  assert.ok(registration, "registration request was not sent");
  assert.equal(registration.body.username, "tiger_trader");
  assert.equal(registration.body.email, "trader@tigerswap.local");
});

test("register trims surrounding whitespace from the username", async () => {
  requests.length = 0;
  await register("trader@tigerswap.local", "Str0ng-Passw0rd!2026", "  tiger_trader  ");
  const registration = requests.find((entry) => entry.path.endsWith("/auth/register"));
  assert.equal(registration?.body.username, "tiger_trader");
});

test("register rejects an unsafe username before calling the server", async () => {
  requests.length = 0;
  await assert.rejects(
    () => register("trader@tigerswap.local", "Str0ng-Passw0rd!2026", "<script>alert(1)</script>"),
  );
  assert.equal(requests.length, 0, "no request should reach the server");
});

test("register completes a full session", async () => {
  const user = await register("trader@tigerswap.local", "Str0ng-Passw0rd!2026", "tiger_trader");
  assert.equal(user.email, "trader@tigerswap.local");
  assert.equal(getCurrentUser()?.id, "u1");
});

test("login stores the session and logout clears it", async () => {
  const user = await login("trader@tigerswap.local", "Str0ng-Passw0rd!2026");
  assert.equal(user.role, "user");
  assert.equal(getCurrentUser()?.id, "u1");

  await logout();
  assert.equal(getCurrentUser(), null);
  setSessionToken(null);
});
