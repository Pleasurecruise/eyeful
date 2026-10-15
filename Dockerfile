FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY workflow/go.mod workflow/go.sum workflow/
COPY local/go.mod local/go.sum local/
RUN go mod download
COPY cmd cmd
COPY internal internal
COPY workflow workflow
COPY local local
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /eyeful-server ./cmd/eyeful-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /eyeful-server /eyeful-server
EXPOSE 8080
ENTRYPOINT ["/eyeful-server", "serve"]
