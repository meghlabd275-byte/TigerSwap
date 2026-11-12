# syntax=docker/dockerfile:1

# Build stage: produce a Next.js standalone server bundle.
FROM node:20-bookworm-slim AS build

WORKDIR /app

# Install dependencies. package-lock.json is gitignored in this repository, so
# it may be absent from a clean checkout; npm install matches the CI workflow.
COPY package.json ./
RUN npm install

COPY . .

# The API base is baked into the client bundle at build time, so it must be
# supplied here rather than at runtime. NEXT_PUBLIC_WS_URL is optional and
# derived from the API base when unset.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ARG NEXT_PUBLIC_WS_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_WS_URL=$NEXT_PUBLIC_WS_URL
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# Runtime stage: only the standalone output, static assets and public files.
FROM node:20-bookworm-slim AS runtime

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN useradd --system --uid 10001 --home /app nextjs

COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nextjs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nextjs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

CMD ["node", "server.js"]
