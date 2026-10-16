FROM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY workflow/go.mod workflow/go.sum workflow/
COPY local/go.mod local/go.sum local/
RUN go mod download
COPY cmd cmd
COPY internal internal
COPY workflow workflow
COPY local local
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /eyeful-server ./cmd/eyeful-server

FROM node:24-bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends git ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& npm install --global @earendil-works/pi-coding-agent@1.3.0
COPY --from=build /eyeful-server /usr/local/bin/eyeful-server
EXPOSE 8080
ENTRYPOINT ["eyeful-server", "serve"]
