FROM node:26-slim AS web
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml openapi-ts.config.ts ./
COPY apps/web apps/web
COPY packages/sdk packages/sdk
COPY packages/ui packages/ui
RUN pnpm install --frozen-lockfile --filter @eyeful/web...
RUN pnpm --filter @eyeful/web build

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY local/go.mod local/go.sum local/
COPY workflow/go.mod workflow/go.sum workflow/
RUN go mod download
COPY . .
COPY --from=web /src/apps/web/dist apps/web/dist
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /eyeful-server ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /eyeful-server /eyeful-server
EXPOSE 8080
ENTRYPOINT ["/eyeful-server", "serve"]
