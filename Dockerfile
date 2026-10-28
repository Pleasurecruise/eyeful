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
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /eyeful-action ./cmd/eyeful-action

FROM node:26-bookworm-slim
RUN apt-get update \
	&& apt-get install -y --no-install-recommends git ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& npm install --global --prefix /opt/pi @earendil-works/pi-coding-agent@1.4.0 \
	&& useradd --create-home agent \
	&& printf '#!/bin/sh\nexec env HOME=/home/agent setpriv --reuid=agent --regid=agent --init-groups --inh-caps=+dac_override --ambient-caps=+dac_override /opt/pi/bin/pi "$@"\n' > /usr/local/bin/pi \
	&& chmod 755 /usr/local/bin/pi
COPY --chown=agent:agent cmd/eyeful-action/pi/ /home/agent/.pi/agent/
COPY --from=build /eyeful-action /usr/local/bin/eyeful-action
ENTRYPOINT ["eyeful-action"]
