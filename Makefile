.PHONY: deploy-local deploy-local-isp deploy-isp-only deploy-with-external-isp verify-isp test monitor clean clean-isp validate help status tinc-bootstrap
.PHONY: test-fast test-env test-configs test-builds test-integration test-e2e test-all
.PHONY: test-isp-integrated test-isp-external

deploy-local: ## Deploy local environment (mesh only)
	docker compose up -d --build

deploy-local-isp: ## Deploy mesh + ISP (integrated mode)
	@echo "=== Deploying mesh + ISP via profile ==="
	ISP_ENABLED=true docker compose --profile isp up -d --build

deploy-isp-only: ## Deploy standalone ISP
	@echo "=== Deploying standalone ISP ==="
	docker compose -f docker-compose.isp.yml up -d --build

deploy-with-external-isp: ## Deploy mesh with external ISP (for Host B)
	@echo "=== Deploying mesh with external ISP connectivity ==="
	@echo "Make sure ISP_ENABLED=true and ISP_NEIGHBOR=<Host_A_IP> are set in .env"
	docker compose -f docker-compose.yml -f docker-compose.external-isp.yml up -d --build

verify-isp: ## Verify external ISP BGP session
	@echo "=== Verifying ISP BGP session ==="
	@docker exec bird1 birdc show protocols isp
	@echo ""
	@echo "=== ISP routes received ==="
	@docker exec bird1 birdc show route protocol isp

test: ## Run integration tests
	./tests/integration/test_bgp_peering.sh

monitor: ## Open monitoring dashboard
	@echo "Opening Grafana at http://localhost:3000"
	@echo "Opening Prometheus at http://localhost:9090"
	@xdg-open http://localhost:3000 2>/dev/null || open http://localhost:3000 2>/dev/null || echo "Please open http://localhost:3000 manually"

clean: ## Clean up mesh deployment
	docker compose down -v

clean-isp: ## Clean up ISP deployment (standalone)
	docker compose -f docker-compose.isp.yml down -v

clean-all: ## Clean up everything (mesh + ISP)
	docker compose down -v
	docker compose -f docker-compose.isp.yml down -v 2>/dev/null || true
	docker network rm bgp-isp-net 2>/dev/null || true

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

test-isp-integrated: ## Test mesh + ISP integration
	@./tests/integration/test_isp_integrated.sh

test-isp-external: ## Test with external ISP
	@ISP_EXTERNAL=true ./tests/integration/test_isp_external.sh

test-all: test-fast test-integration test-e2e ## Run all tests

test-all-isp: test-fast test-integration test-isp-integrated ## Run all tests including ISP

status: ## Show status of all containers
	@docker ps --format "table {{.Names}}\t{{.Status}}" | grep -E "NAME|bird|tinc|etcd|prom"

tinc-bootstrap: ## Bootstrap TINC mesh connectivity
	./tinc_bootstrap.sh

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
