LIBS := wallet

.PHONY: test build tidy vet publish all $(LIBS)

## Run tests for all libs that have a go.mod
test:
	@for lib in $(LIBS); do \
		if [ -f libs/$$lib/go.mod ]; then \
			echo ">>> testing $$lib"; \
			cd libs/$$lib && go test ./... && cd ../..; \
		fi \
	done

## Build all libs that have a go.mod
build:
	@for lib in $(LIBS); do \
		if [ -f libs/$$lib/go.mod ]; then \
			echo ">>> building $$lib"; \
			cd libs/$$lib && go build ./... && cd ../..; \
		fi \
	done

## Run go vet for all libs
vet:
	@for lib in $(LIBS); do \
		if [ -f libs/$$lib/go.mod ]; then \
			echo ">>> vetting $$lib"; \
			cd libs/$$lib && go vet ./... && cd ../..; \
		fi \
	done

## Run go mod tidy for all libs
tidy:
	@for lib in $(LIBS); do \
		if [ -f libs/$$lib/go.mod ]; then \
			echo ">>> tidying $$lib"; \
			cd libs/$$lib && go mod tidy && cd ../..; \
		fi \
	done

## Run test for a single lib: make lib=wallet one
one:
	cd libs/$(lib) && go test ./...

## Publish all libs at a new version: make publish version=0.0.2 [core_version=0.0.2]
##   - Pins gox-packages/libs/{core,blobs} requires across every lib to
##     core_version (defaults to version).
##   - Updates the workspace replace in go.work to match.
##   - Commits + pushes main, tags every lib, then pushes all tags.
## Note: gox-packages must already be published at core_version on the remote.
publish:
	@test -n "$(version)" || { echo "ERROR: version is required (e.g. make publish version=0.0.2)"; exit 1; }
	@case "$(version)" in v*) echo "ERROR: omit the leading 'v' (got '$(version)')"; exit 1;; esac
	@CV="$(if $(core_version),$(core_version),$(version))"; \
	case "$$CV" in v*) echo "ERROR: core_version must omit leading 'v'"; exit 1;; esac; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: working tree is not clean. Commit or stash first."; exit 1; \
	fi; \
	echo ">>> Pinning gox-packages/libs/{core,blobs} requires to v$$CV"; \
	for lib in $(LIBS); do \
		perl -i -pe 's{(github\.com/real-legger/gox-packages/libs/(?:core|blobs) )v[0-9][\S]*}{$$1v'"$$CV"'}' libs/$$lib/go.mod; \
	done; \
	echo ">>> Updating go.work replace lines to v$$CV"; \
	perl -i -pe 's{(github\.com/real-legger/gox-packages/libs/(?:core|blobs) )v[0-9][\S]*( =>)}{$$1v'"$$CV"'$$2}' go.work; \
	if [ -n "$$(git status --porcelain)" ]; then \
		git add libs/*/go.mod go.work; \
		git commit -m "Release v$(version)"; \
		echo ">>> Committed version bump"; \
	fi; \
	echo ">>> Pushing main"; \
	git push origin HEAD; \
	for lib in $(LIBS); do \
		echo ">>> Tagging libs/$$lib/v$(version)"; \
		git tag libs/$$lib/v$(version); \
	done; \
	git push origin --tags; \
	echo ">>> Published v$(version) for: $(LIBS)"

## Show which libs have been scaffolded
status:
	@echo "Scaffolded libs:"; \
	for lib in $(LIBS); do \
		if [ -f libs/$$lib/go.mod ]; then \
			echo "  [x] $$lib"; \
		else \
			echo "  [ ] $$lib"; \
		fi \
	done
