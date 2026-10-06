# Both build stages run on the build platform: web assets are arch-independent and Go cross-compiles.
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ui/.npmrc ./
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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags "-s -w" -o /kinora ./cmd/kinora

FROM alpine:3
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 kinora
COPY --from=build /kinora /usr/local/bin/kinora
USER kinora
ENV ADDR=:8080
EXPOSE 8080
HEALTHCHECK CMD wget -qO- http://localhost:8080/api/v1/setup >/dev/null || exit 1
ENTRYPOINT ["kinora"]
