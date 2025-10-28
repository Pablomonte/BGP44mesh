.PHONY: deploy-local test monitor clean validate help
.PHONY: test-fast test-env test-configs test-builds test-integration test-e2e test-all

deploy-local: ## Deploy local environment
	docker-compose up -d --build

test: ## Run integration tests
	./tests/integration/test_bgp_peering.sh

monitor: ## Open monitoring dashboard
	@echo "Opening Grafana at http://localhost:3000"
	@echo "Opening Prometheus at http://localhost:9090"
	@xdg-open http://localhost:3000 2>/dev/null || open http://localhost:3000 2>/dev/null || echo "Please open http://localhost:3000 manually"

clean: ## Clean up
	docker-compose down -v

validate: ## Validate configs
	@if [ -d ansible ]; then ansible-playbook ansible/site.yml --syntax-check; else echo "Ansible not yet implemented"; fi

# Test targets
test-fast: ## Run fast validation tests (parallel)
	@echo "=== Running fast validation tests ==="
	$(MAKE) -j4 test-env test-configs test-builds

test-env: ## Test environment variables
	@./tests/validation/test_env_vars.sh

test-configs: ## Test configuration templates
	@./tests/validation/test_configs.sh

test-builds: ## Test Docker builds
	@./tests/validation/test_docker_builds.sh

test-integration: ## Run integration tests
	@./tests/integration/test_bgp_peering.sh

test-e2e: ## Run end-to-end tests
	@./tests/e2e/test_full_stack.sh

test-all: test-fast test-integration test-e2e ## Run all tests

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
