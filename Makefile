.PHONY: help status clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

status: ## Show status of containers
	@docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "NAME|bird|netmaker|netclient|mq" || echo "No containers running"

clean: ## Stop and remove all project containers
	@for dir in deploy/*/; do \
		echo "Cleaning $$dir..."; \
		(cd "$$dir" && docker compose down -v 2>/dev/null) || true; \
	done

.DEFAULT_GOAL := help

