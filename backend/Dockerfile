# syntax=docker/dockerfile:1.19@sha256:b6afd42430b15f2d2a4c5a02b919e98a525b785b1aaff16747d2f623364e39b6

FROM node:22.22.0-alpine3.23@sha256:e4bf2a82ad0a4037d28035ae71529873c069b13eb0455466ae0bc13363826e34 AS node-runtime

FROM node-runtime AS runner-build
WORKDIR /src/pi-runner
RUN corepack enable && corepack prepare pnpm@11.13.1 --activate
COPY pi-runner/package.json pi-runner/pnpm-lock.yaml pi-runner/pnpm-workspace.yaml ./
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile
COPY pi-runner/tsconfig.json pi-runner/tsconfig.build.json ./
COPY pi-runner/src ./src
RUN pnpm run build && pnpm prune --prod

FROM golang:1.25.0-alpine3.22@sha256:f18a072054848d87a8077455f0ac8a25886f2397f88bfdd222d6fafbb5bba440 AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/todai ./cmd/todai

FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.opencontainers.image.title="Todai backend" \
      org.opencontainers.image.description="Todai Go backend with the bundled Pi runner" \
      org.opencontainers.image.source="https://github.com/mishankov/todai" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}"

RUN apk add --no-cache ca-certificates libstdc++ \
    && addgroup -S -g 10001 todai \
    && adduser -S -D -H -u 10001 -G todai -h /var/lib/todai todai \
    && mkdir -p /var/lib/todai/pi-agent /opt/todai/pi-runner \
    && chown -R todai:todai /var/lib/todai /opt/todai
COPY --from=backend-build /out/todai /usr/local/bin/todai
COPY --from=node-runtime /usr/local/bin/node /usr/local/bin/node
COPY --from=runner-build --chown=todai:todai /src/pi-runner/dist /opt/todai/pi-runner/dist
COPY --from=runner-build --chown=todai:todai /src/pi-runner/node_modules /opt/todai/pi-runner/node_modules
COPY --from=runner-build --chown=todai:todai /src/pi-runner/package.json /opt/todai/pi-runner/package.json

ENV HOME=/var/lib/todai \
    NODE_ENV=production \
    TODAI_ENVIRONMENT=production \
    TODAI_HTTP_PORT=8080 \
    TODAI_LOG_FORMAT=json \
    TODAI_RUNNER_EXECUTABLE=/usr/local/bin/node \
    TODAI_RUNNER_ENTRY=/opt/todai/pi-runner/dist/cli/main.js \
    TODAI_PI_AGENT_DIR=/var/lib/todai/pi-agent
WORKDIR /var/lib/todai
USER todai
EXPOSE 8080
STOPSIGNAL SIGTERM
ENTRYPOINT ["/usr/local/bin/todai"]
CMD ["run"]
