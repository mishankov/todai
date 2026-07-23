# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

FROM node:22.22.0-alpine3.23@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 AS node-runtime

FROM node-runtime AS frontend-build
WORKDIR /src/frontend
RUN corepack enable && corepack prepare pnpm@11.13.1 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml frontend/.npmrc ./
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile
COPY frontend/ ./
RUN pnpm run build

FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.opencontainers.image.title="Todai frontend" \
      org.opencontainers.image.description="Todai web application and same-origin streaming API proxy" \
      org.opencontainers.image.source="https://github.com/mishankov/todai" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}"

RUN apk add --no-cache ca-certificates libstdc++ \
    && addgroup -S -g 10002 todai-web \
    && adduser -S -D -H -u 10002 -G todai-web -h /opt/todai/frontend todai-web \
    && mkdir -p /opt/todai/frontend \
    && chown -R todai-web:todai-web /opt/todai
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
COPY --from=frontend-build --chown=todai-web:todai-web /src/frontend/build /opt/todai/frontend/build
COPY --chown=todai-web:todai-web frontend/server.mjs /opt/todai/frontend/server.mjs

ENV NODE_ENV=production \
    HOST=0.0.0.0 \
    PORT=8080 \
    TODAI_BACKEND_URL=http://backend:8080
WORKDIR /opt/todai/frontend
USER todai-web
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/node", "/opt/todai/frontend/server.mjs"]
