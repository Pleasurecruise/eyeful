FROM node:26-slim AS web
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY cloud/web cloud/web
COPY cloud/sdk cloud/sdk
COPY packages/ui packages/ui
RUN pnpm install --frozen-lockfile --filter @eyeful/web...
RUN pnpm --filter @eyeful/web build

FROM golang:1.27 AS build
WORKDIR /src
COPY cloud/go.mod cloud/go.sum cloud/
COPY workflow/go.mod workflow/go.sum workflow/
RUN cd cloud && go mod download
COPY workflow workflow
COPY cloud cloud
COPY --from=web /src/cloud/web/dist cloud/web/dist
ARG VERSION=dev
WORKDIR /src/cloud
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /eyeful-server .

FROM gcr.io/distroless/static-debian12
COPY --from=build /eyeful-server /eyeful-server
EXPOSE 8080
ENTRYPOINT ["/eyeful-server", "serve"]
