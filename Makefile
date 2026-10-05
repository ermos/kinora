BINARY := kinora
SWAG   := go run github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5

.PHONY: build build-web ui run run-dev dev test test-live lint fmt openapi openapi-check docker

## build: compile the server (embeds whatever is in ui/dist)
build:
	CGO_ENABLED=0 go build -ldflags "-s -w" -o bin/$(BINARY) ./cmd/kinora

## ui: export the Expo app for the web into ui/dist
ui:
	cd ui && npm ci && npm run build

## build-web: web export then binary with the UI embedded
build-web: ui build

## run: run the server (needs TMDB_API_KEY)
run:
	go run ./cmd/kinora

## dev: Expo dev server with hot reload, open http://localhost:8080 (run `make run-dev` alongside)
dev:
	cd ui && npx expo start --web --port 8081

## run-dev: the server, proxying the UI to the Expo dev server
run-dev:
	UI_DEV_URL=http://localhost:8081 go run ./cmd/kinora

test:
	go test -race ./...

## test-live: check every ported source and hoster against the real sites
test-live:
	go test -tags live -v -count=1 ./internal/scraper -run Live

lint:
	golangci-lint run ./...
	cd ui && npx tsc --noEmit

fmt:
	gofmt -s -w .

## openapi: regenerate the spec from handler annotations, then the typed TS client
openapi:
	$(SWAG) init -g doc.go -d internal/api,internal/store,internal/tmdb -o internal/api/docs \
	  --ot json,yaml --parseInternal --requiredByDefault --v3.1
	cd ui && npm run gen:api

## openapi-check: fail if the committed spec is stale (CI)
openapi-check: openapi
	git diff --exit-code internal/api/docs ui/src/api/schema.ts

docker:
	docker build -t kinora .
