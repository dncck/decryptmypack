.PHONY: build dev pages-build deploy-workers deploy-pages deploy deploy-local

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
	attempt=1; \
	until [ "$$attempt" -gt 3 ]; do \
		echo "Pages deploy attempt $$attempt/3"; \
		if npx wrangler pages deploy frontend/dist --project-name decryptmypack --commit-dirty=true; then \
			exit 0; \
		fi; \
		if [ "$$attempt" -eq 3 ]; then \
			echo "Pages deploy failed after $$attempt attempts"; \
			exit 1; \
		fi; \
		sleep_seconds=$$((attempt * 10)); \
		echo "Retrying Pages deploy in $${sleep_seconds}s..."; \
		sleep "$$sleep_seconds"; \
		attempt=$$((attempt + 1)); \
	done

deploy: deploy-workers deploy-pages

deploy-local: pages-build
	npx wrangler pages dev frontend/dist
