/**
 * Security headers for every response.
 *
 * The Content-Security-Policy is built around a per-request nonce. Next.js
 * reads the `Content-Security-Policy` request header, extracts the nonce and
 * stamps it onto the framework's own script tags, so no inline script runs
 * without it. `strict-dynamic` then lets those trusted scripts load the rest of
 * the bundle while an injected `<script>` from a compromised data source has
 * nothing to execute under. This is what makes an XSS payload inert rather than
 * merely unlikely: even if a value reached the DOM unescaped, the browser
 * refuses to run it.
 *
 * `style-src` keeps `unsafe-inline` because Tailwind and Next inject style
 * attributes and style tags at runtime; styles cannot exfiltrate a session the
 * way scripts can, and the token is never written to storage.
 */

import { NextResponse, type NextRequest } from "next/server";

const isProduction = process.env.NODE_ENV === "production";

/** Wallet RPC and API origins the app is allowed to reach. */
function connectSources(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
  const sources = new Set<string>(["'self'", "https:", "wss:"]);
  if (configured) {
    try {
      const url = new URL(configured);
      sources.add(url.origin);
    } catch {
      // A relative base such as "/api/v1" is already covered by 'self'.
    }
  }
  return [...sources].join(" ");
}

function buildCsp(nonce: string): string {
  const directives = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'`,
    "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
    "font-src 'self' https://fonts.gstatic.com data:",
    "img-src 'self' data: blob: https:",
    `connect-src ${connectSources()}`,
    "frame-ancestors 'none'",
    "frame-src 'none'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "manifest-src 'self'",
    "worker-src 'self' blob:",
  ];
  if (isProduction) {
    directives.push("upgrade-insecure-requests");
  }
  return directives.join("; ");
}

export function middleware(request: NextRequest): NextResponse {
  // 16 random bytes, base64 encoded: enough entropy that a nonce cannot be
  // guessed or reused by an attacker who observes one response.
  const nonce = Buffer.from(crypto.getRandomValues(new Uint8Array(16))).toString("base64");
  const csp = buildCsp(nonce);

  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  // Next.js reads this header to apply the nonce to its own script tags.
  requestHeaders.set("Content-Security-Policy", csp);

  const response = NextResponse.next({ request: { headers: requestHeaders } });

  response.headers.set("Content-Security-Policy", csp);
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");
  response.headers.set("X-DNS-Prefetch-Control", "off");
  response.headers.set(
    "Permissions-Policy",
    "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
  );
  response.headers.set("Cross-Origin-Opener-Policy", "same-origin");
  response.headers.set("Cross-Origin-Resource-Policy", "same-origin");
  if (isProduction) {
    response.headers.set(
      "Strict-Transport-Security",
      "max-age=31536000; includeSubDomains",
    );
  }

  return response;
}

export const config = {
  // Everything except Next's static assets, which are immutable and carry no
  // user data.
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
