.PHONY: build dev pages-build deploy-workers deploy-pages deploy

build:
	env GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go build .

dev:
	go run . dev

pages-build:
	./frontend/build-pages.sh

deploy-workers:
	cd workers/packs && npx wrangler deploy
	cd workers/download-api && npx wrangler deploy

deploy-pages: pages-build
	npx wrangler pages deploy frontend/dist --project-name decryptmypack --commit-dirty=true

deploy: deploy-workers deploy-pages
