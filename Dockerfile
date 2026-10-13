# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/epg-vmedia ./cmd/epg-vmedia \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/import-channels ./cmd/import-channels \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/generate-epg ./cmd/generate-epg

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 10001 epg \
 && adduser -S -D -H -u 10001 -G epg epg \
 && mkdir -p /data /opt/epg-vmedia/templates \
 && chown -R epg:epg /data /opt/epg-vmedia

WORKDIR /opt/epg-vmedia
COPY --from=build --chown=epg:epg /out/epg-vmedia /opt/epg-vmedia/epg-vmedia
COPY --from=build --chown=epg:epg /out/import-channels /opt/epg-vmedia/import-channels
COPY --from=build --chown=epg:epg /out/generate-epg /opt/epg-vmedia/generate-epg
COPY --chown=epg:epg templates/ /opt/epg-vmedia/templates/

ENV EPG_HOST=0.0.0.0 \
    EPG_PORT=5580 \
    EPG_DATA_DIR=/data \
    EPG_XML_PATH=/data/epg.xml \
    TZ=America/Toronto

EXPOSE 5580
VOLUME ["/data"]

USER epg
ENTRYPOINT ["/opt/epg-vmedia/epg-vmedia"]
