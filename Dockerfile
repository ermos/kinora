# Both build stages run on the build platform: web assets are arch-independent and Go cross-compiles.
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=ui /src/ui/dist ./ui/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-s -w" -o /istream ./cmd/istream

FROM alpine:3
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 istream && mkdir /data && chown istream /data
COPY --from=build /istream /usr/local/bin/istream
USER istream
ENV DATA_DIR=/data ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK CMD wget -qO- http://localhost:8080/api/v1/setup >/dev/null || exit 1
ENTRYPOINT ["istream"]
