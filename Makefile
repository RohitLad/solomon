# Solomon — one place for every command (so nobody memorizes anything).
# Run `make` or `make help` to list targets.
#
# Two ways to develop:
#   make dev      split-terminal mode: prints one command per pane, runs nothing.
#   make dev-bg   background mode: starts both services, logs to .logs/, `make stop` ends them.

BACKEND_DIR  := backend
FRONTEND_DIR := frontend
API_PORT     := 8080
WEB_PORT     := 5173
API_URL      := http://localhost:$(API_PORT)
PID_FILE     := .make.pids
LOG_DIR      := .logs

.PHONY: help install env dev dev-bg dev-backend dev-frontend \
        build vet test health smoke smoke-live stop clean

help: ## Show this help (default).
	@echo "Solomon targets:"
	@echo "  make install      Install Go modules + npm packages"
	@echo "  make env          Create backend/.env from .env.example (never overwrites)"
	@echo "  make dev          SPLIT-TERMINAL mode: print one command per pane (runs nothing)"
	@echo "  make dev-bg       BACKGROUND mode: start API + web in background (logs: $(LOG_DIR)/)"
	@echo "  make dev-backend  Run API in foreground (:$(API_PORT))"
	@echo "  make dev-frontend Run web UI in foreground (:5173)"
	@echo "  make build        Production builds (backend binary + frontend dist/)"
	@echo "  make vet          go vet + svelte-check"
	@echo "  make test         Backend (go test) + frontend (vitest) suites"
	@echo "  make health       Check the API is up (fails if down)"
	@echo "  make smoke        End-to-end API smoke test on throwaway DB + port 18081"
	@echo "  make smoke-live   LIVE-FIRE test: labeled posts ONLY to accounts with smoke=true in Extra"
	@echo "  make stop         Stop background services started by dev-bg"
	@echo "  make clean        Remove build artifacts, throwaway DBs, logs (keeps solomon.db)"

install: ## Install Go modules + npm packages.
	cd $(BACKEND_DIR) && go mod tidy
	cd $(FRONTEND_DIR) && npm install

env: ## Create backend/.env from .env.example (refuses to overwrite).
	@if [ -f $(BACKEND_DIR)/.env ]; then \
		echo "$(BACKEND_DIR)/.env already exists — leaving it alone."; \
	else \
		cp $(BACKEND_DIR)/.env.example $(BACKEND_DIR)/.env; \
		echo "Created $(BACKEND_DIR)/.env — fill in client IDs/secrets to go live."; \
	fi

dev: ## SPLIT-TERMINAL mode: show the two commands, run nothing.
	@echo "Open TWO terminal panes and run one line in each:"
	@echo ""
	@echo "  pane 1 (API):  cd $(BACKEND_DIR) && go run ."
	@echo "  pane 2 (web):  cd $(FRONTEND_DIR) && npm run dev"
	@echo ""
	@echo "Then open http://localhost:$(WEB_PORT) (API on $(API_URL))."

dev-bg: ## BACKGROUND mode: start API + web in background; `make stop` ends them.
	@mkdir -p $(LOG_DIR)
	@if [ -f $(PID_FILE).backend ] || [ -f $(PID_FILE).frontend ]; then \
		echo "Already running (see $(PID_FILE).backend / $(PID_FILE).frontend). Run 'make stop' first."; exit 1; fi
	cd $(BACKEND_DIR) && (nohup go run . > ../$(LOG_DIR)/backend.log 2>&1 & echo $$! > ../$(PID_FILE).backend)
	cd $(FRONTEND_DIR) && (nohup npm run dev > ../$(LOG_DIR)/frontend.log 2>&1 & echo $$! > ../$(PID_FILE).frontend)
	@for i in $$(seq 1 30); do curl -sf $(API_URL)/api/health >/dev/null 2>&1 && break; sleep 1; done; \
	if ! curl -sf $(API_URL)/api/health >/dev/null 2>&1; then \
		echo "Backend did not start (see $(LOG_DIR)/backend.log). Stopping."; $(MAKE) stop >/dev/null 2>&1; exit 1; fi
	@if ! curl -sf http://localhost:$(WEB_PORT)/ >/dev/null 2>&1; then \
		echo "note: web UI not up yet - check $(LOG_DIR)/frontend.log"; fi
	@echo "Started. Logs: $(LOG_DIR)/backend.log $(LOG_DIR)/frontend.log"
	@echo "Web: http://localhost:$(WEB_PORT)   API: $(API_URL)/api/health"
	@echo "Stop with: make stop"

dev-backend: ## Run the API in the foreground.
	cd $(BACKEND_DIR) && go run .

dev-frontend: ## Run the web UI in the foreground.
	cd $(FRONTEND_DIR) && npm run dev

build: ## Production builds: backend binary + frontend dist/.
	cd $(BACKEND_DIR) && go build -o solomon .
	cd $(FRONTEND_DIR) && npm run build
	@echo "Built: backend/solomon + frontend/dist/"

vet: ## Static checks: go vet + svelte-check.
	cd $(BACKEND_DIR) && go vet ./...
	cd $(FRONTEND_DIR) && npm run check

test: ## Backend (go test) + frontend (vitest) suites.
	cd $(BACKEND_DIR) && go test ./...
	cd $(FRONTEND_DIR) && npm run test

health: ## Curl the API health endpoint (fails if the API is down).
	curl -sf $(API_URL)/api/health && echo

SMOKE_PORT := 18081
SMOKE_DB   := /tmp/solomon_smoke.db
SMOKE_URL  := http://localhost:$(SMOKE_PORT)

smoke: ## Full API smoke test (throwaway DB + port; never touches your data).
	@rm -f $(SMOKE_DB); \
	trap 'kill $$(cat /tmp/solomon_smoke.pid 2>/dev/null) 2>/dev/null || true; lsof -ti:$(SMOKE_PORT) 2>/dev/null | xargs kill -9 2>/dev/null || true; rm -f $(SMOKE_DB) /tmp/solomon_smoke.csv /tmp/solomon_smoke.pid /tmp/solomon_smoke.log' EXIT INT TERM; \
	cd $(BACKEND_DIR) && (PORT=$(SMOKE_PORT) DB_PATH=$(SMOKE_DB) nohup go run . > /tmp/solomon_smoke.log 2>&1 & echo $$! > /tmp/solomon_smoke.pid) && \
	for i in $$(seq 1 30); do curl -sf $(SMOKE_URL)/api/health >/dev/null 2>&1 && break; sleep 1; done && \
	curl -sf $(SMOKE_URL)/api/health >/dev/null || { echo "API did not start; see /tmp/solomon_smoke.log"; exit 1; } && \
	TW=$$(curl -sf -X POST $(SMOKE_URL)/api/accounts -H 'Content-Type: application/json' -d '{"network":"twitter","name":"@smoke"}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert 'avatar_url' in d, d;print(d['id'])") && \
	[ -n "$$TW" ] || { echo "account create failed"; exit 1; } && \
	echo "accounts: OK ($$TW)" && \
	curl -sf -X POST $(SMOKE_URL)/api/ai/captions -H 'Content-Type: application/json' -d '{"text":"smoke test post","network":"twitter"}' >/dev/null && echo "captions: OK" && \
	curl -sf "$(SMOKE_URL)/api/schedule/suggest?networks=twitter&count=1" >/dev/null && echo "suggest: OK" && \
	LONG=$$(python3 -c "print('lorem ipsum dolor sit amet '*30)") && \
	PUBID=$$(curl -sf -X POST $(SMOKE_URL)/api/posts -H 'Content-Type: application/json' -d "{\"content\":\"$$LONG\",\"publish_now\":true,\"targets\":[{\"account_id\":\"$$TW\"}]}" | python3 -c "import json,sys;print(json.load(sys.stdin)['id'])") && \
	[ -n "$$PUBID" ] || { echo "no published post returned"; exit 1; } && echo "publish(thread): OK ($$PUBID)" && \
	curl -sf -X POST $(SMOKE_URL)/api/analytics/refresh >/dev/null && echo "analytics: OK" && \
	printf 'title,content,link,scheduled_at,accounts\n"T1","Bulk one","https://example.com","","twitter"\n' > /tmp/solomon_smoke.csv && \
	curl -sf -X POST $(SMOKE_URL)/api/posts/bulk -F "file=@/tmp/solomon_smoke.csv" >/dev/null && echo "bulk: OK" && \
	PID=$$(curl -sf $(SMOKE_URL)/api/posts | python3 -c "import json,sys;print(json.load(sys.stdin)[0]['id'])") && \
	[ -n "$$PID" ] || { echo "no posts returned"; exit 1; } && \
	curl -sf -X POST $(SMOKE_URL)/api/evergreen -H 'Content-Type: application/json' -d "{\"name\":\"smoke\",\"pool_post_ids\":[\"$$PID\"],\"account_ids\":[\"$$TW\"],\"interval_hours\":24}" >/dev/null && echo "evergreen: OK" && \
	curl -sf -X POST $(SMOKE_URL)/api/accounts/refresh >/dev/null && echo "tokens: OK" && \
	DPID=$$(curl -sf -X POST $(SMOKE_URL)/api/posts -H 'Content-Type: application/json' -d "{\"content\":\"cal draft\",\"targets\":[{\"account_id\":\"$$TW\"}]}" | python3 -c "import json,sys;print(json.load(sys.stdin)['id'])") && \
	[ -n "$$DPID" ] || { echo "no draft post returned"; exit 1; } && \
	curl -sf -X PATCH $(SMOKE_URL)/api/posts/$$DPID -H 'Content-Type: application/json' -d '{"scheduled_at":"2031-01-01T10:00:00Z"}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert d['status']=='scheduled' and '2031' in (d.get('scheduled_at') or ''), d" && echo "reschedule: OK" && \
	curl -sf -X DELETE $(SMOKE_URL)/api/posts/$$DPID | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('soft_deleted') is False, d" && echo "delete(draft,hard): OK" && \
	curl -sf -X DELETE $(SMOKE_URL)/api/posts/$$PUBID | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('soft_deleted') is True, d" && echo "delete(published,soft): OK" && \
	curl -sf $(SMOKE_URL)/api/analytics | python3 -c "import json,sys;d=json.load(sys.stdin);rs=d['rows'];assert any(r.get('deleted') for r in rs),'no deleted row';assert any(r.get('external') for r in rs),'no external row';assert d.get('outside',{}).get('posts',0)>=1,'no outside totals'" && echo "analytics(history+outside): OK" && \
	echo "SMOKE PASSED"

LIVE_PORT := 18082
LIVE_DB   := /tmp/solomon_smoke_live.db
LIVE_URL  := http://localhost:$(LIVE_PORT)

smoke-live: ## LIVE-FIRE test against real tokens (copy of your DB; publishes ONLY to smoke=true accounts).
	@if [ ! -f $(BACKEND_DIR)/solomon.db ]; then \
		echo "No $(BACKEND_DIR)/solomon.db yet — run the app once and add your accounts first."; exit 1; fi
	@rm -f $(LIVE_DB) && cp $(BACKEND_DIR)/solomon.db $(LIVE_DB) && \
	trap 'kill $$(cat /tmp/solomon_smoke_live.pid 2>/dev/null) 2>/dev/null || true; lsof -ti:$(LIVE_PORT) 2>/dev/null | xargs kill -9 2>/dev/null || true; rm -f $(LIVE_DB) /tmp/solomon_smoke_live.pid /tmp/solomon_smoke_live.log' EXIT INT TERM && \
	cd $(BACKEND_DIR) && (PORT=$(LIVE_PORT) DB_PATH=$(LIVE_DB) nohup go run . > /tmp/solomon_smoke_live.log 2>&1 & echo $$! > /tmp/solomon_smoke_live.pid) && \
	for i in $$(seq 1 30); do curl -sf $(LIVE_URL)/api/health >/dev/null 2>&1 && break; sleep 1; done && \
	curl -sf $(LIVE_URL)/api/health >/dev/null || { echo "API did not start; see /tmp/solomon_smoke_live.log"; exit 1; } && \
	TEST_ACCTS=$$(curl -sf $(LIVE_URL)/api/accounts | python3 -c "import json,sys; print('\n'.join(f\"{a['network']}|{a['name']}|{a['id']}\" for a in json.load(sys.stdin) if any(k in (a.get('extra') or '').lower() for k in ('smoke=true','test=true','sandbox=true'))))") && \
	if [ -z "$$TEST_ACCTS" ]; then \
		echo "ABORT: no test accounts. Flag a SANDBOX account first:"; \
		echo "  1. Re-add/duplicate the account with Extra: smoke=true"; \
		echo "     (e.g. Extra 'smoke=true' or 'board_id=123,smoke=true')"; \
		echo "  2. Never flag a production audience account."; \
		exit 1; fi && \
	echo "LIVE test targets (ONLY these accounts will be published to):" && \
	echo "$$TEST_ACCTS" && \
	printf "Type LIVE to publish clearly-labeled test posts to the above: " > /dev/tty 2>/dev/null; read ans < /dev/tty || { echo "ABORT: no confirmation (non-interactive?)."; exit 1; } && \
	if [ "$$ans" != "LIVE" ]; then echo "ABORT: confirmation not given."; exit 1; fi && \
	FIRST_ID=$$(echo "$$TEST_ACCTS" | head -1 | cut -d'|' -f3) && \
	[ -n "$$FIRST_ID" ] || { echo "ABORT: no test account id."; exit 1; } && \
	TARGETS=$$(echo "$$TEST_ACCTS" | cut -d'|' -f3 | python3 -c "import json,sys; print(json.dumps([{'account_id': l.strip()} for l in sys.stdin if l.strip()]))") && \
	NETS=$$(echo "$$TEST_ACCTS" | cut -d'|' -f1 | sort -u | paste -sd, -) && \
	curl -sf -X POST $(LIVE_URL)/api/posts/preview -H 'Content-Type: application/json' -d "{\"content\":\"preview\",\"targets\":[{\"account_id\":\"$$FIRST_ID\"}]}" >/dev/null && echo "preview: OK" && \
	curl -sf -X POST $(LIVE_URL)/api/ai/captions -H 'Content-Type: application/json' -d '{"text":"live smoke test","network":"twitter"}' >/dev/null && echo "captions: OK" && \
	curl -sf "$(LIVE_URL)/api/schedule/suggest?networks=$$NETS&count=1" >/dev/null && echo "suggest: OK" && \
	STAMP=$$(date -u +%Y-%m-%dT%H:%M:%SZ) && \
	RESP=$$(curl -sf -X POST $(LIVE_URL)/api/posts -H 'Content-Type: application/json' -d "{\"content\":\"Solomon live smoke test $$STAMP — safe to delete\",\"publish_now\":true,\"targets\":$$TARGETS}") && \
	echo "$$RESP" | python3 -c "import json,sys; [print(f\"  {t['account']['network']} {t['account']['name']}: {t['status']} {t['network_post_id'] or t['error'][:100]}\") for t in json.load(sys.stdin)['targets']]" && \
	curl -sf -X POST $(LIVE_URL)/api/analytics/refresh >/dev/null && echo "analytics: OK (errors, if any, are per-network scope notes)" && \
	curl -sf $(LIVE_URL)/api/accounts/expiring >/dev/null && echo "expiring: OK (read-only; refresh deliberately skipped — rotation could stale your real DB)" && \
	echo "NOTE: bulk + evergreen are NOT live-tested (they schedule future publishes). Cover them with 'make smoke'." && \
	echo "Clean up the labeled test posts on each network manually (IDs printed above)." && \
	echo "SMOKE-LIVE DONE (copy DB discarded; your real solomon.db untouched)"

stop: ## Stop background services started by `make dev-bg`.
	@if [ -f $(PID_FILE).backend ]; then kill $$(cat $(PID_FILE).backend) 2>/dev/null || true; rm -f $(PID_FILE).backend; echo "backend stopped"; fi
	@if [ -f $(PID_FILE).frontend ]; then kill $$(cat $(PID_FILE).frontend) 2>/dev/null || true; rm -f $(PID_FILE).frontend; echo "frontend stopped"; fi
	@if command -v lsof >/dev/null 2>&1; then \
		lsof -ti:$(API_PORT) 2>/dev/null | xargs kill -9 2>/dev/null || true; \
		lsof -ti:$(WEB_PORT) 2>/dev/null | xargs kill -9 2>/dev/null || true; \
	else echo "(port sweep skipped: lsof not installed)"; fi; true
	@[ -f $(PID_FILE) ] && rm -f $(PID_FILE); true
	@echo "Stopped. (Port sweep covers orphaned 'go run'/vite children.)"

clean: ## Remove build artifacts, throwaway DBs, logs. Keeps backend/solomon.db + uploads/.
	rm -f $(BACKEND_DIR)/solomon /tmp/solomon_smoke.db /tmp/solomon_smoke.* /tmp/solomon_smoke_live.*
	rm -rf $(FRONTEND_DIR)/dist $(LOG_DIR) $(PID_FILE).backend $(PID_FILE).frontend $(PID_FILE)
	@echo "Clean. (Your data: backend/solomon.db + backend/uploads/ untouched.)"
