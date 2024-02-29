include third_party_versions.mk

ROOT?=$(PWD)

EMSDK_DIR=$(ROOT)/third_party/emsdk/upstream/emscripten

DIST_TARGETS=dist/bergamot-translator-worker.wasm dist/bergamot-translator-worker.debug.wasm

.PHONY: lib
lib: $(DIST_TARGETS)

clean:
	rm -rf build dist
clean-lib:
	rm build/*.{js,wasm}
	rm -rf dist

docker-build-bergamot:
	docker build -t bergamot-wasm-builder .
	docker run \
		-u $(id -u):$(id -g) \
		--rm \
		-v $(PWD):/bergamot-wasm \
		-w /bergamot-wasm \
		-e ROOT=/bergamot-wasm \
		bergamot-wasm-builder  \
		make -B lib

build:
	mkdir -p build/

third_party/emsdk: third_party_versions.mk
	mkdir -p third_party/emsdk
	test -d $@/.git || git clone --depth 1 https://github.com/emscripten-core/emsdk.git $@
	cd $@ && git fetch origin $(EMSDK_COMMIT) && git checkout $(EMSDK_COMMIT)
	touch $@

build/emsdk.uptodate: third_party/emsdk | build
	third_party/emsdk/emsdk install latest
	third_party/emsdk/emsdk activate latest
	touch build/emsdk.uptodate

third_party/bergamot: third_party_versions.mk
	mkdir -p third_party/bergamot
	test -d $@/.git || git clone --depth 1 https://github.com/browsermt/bergamot-translator.git $@
	cd $@ && git fetch origin $(BERGAMOT_COMMIT) && git checkout $(BERGAMOT_COMMIT)
	cd $@ && git stash && git apply ../../patches/bergamot.diff
	touch $@

BERGAMOT_CMAKE_OPTIONS=-DCOMPILE_WASM=on -DUSE_THREADS=off

build/bergamot.uptodate: third_party/bergamot build/emsdk.uptodate
	mkdir -p build/bergamot
	(cd build/bergamot && $(EMSDK_DIR)/emcmake cmake $(BERGAMOT_CMAKE_OPTIONS) ../../third_party/bergamot)
	(cd build/bergamot && $(EMSDK_DIR)/emmake make -j2)
	touch build/bergamot.uptodate

build/bergamot-debug.uptodate: third_party/bergamot build/emsdk.uptodate
	mkdir -p build/bergamot-debug
	(cd build/bergamot-debug && $(EMSDK_DIR)/emcmake cmake $(BERGAMOT_CMAKE_OPTIONS) -DCMAKE_BUILD_TYPE=Debug ../../third_party/bergamot)
	(cd build/bergamot-debug && $(EMSDK_DIR)/emmake make -j2)
	touch build/bergamot-debug.uptodate

build/bergamot/bergamot-translator-worker.wasm: build/bergamot.uptodate

build/bergamot-debug/bergamot-translator-worker.wasm: build/bergamot-debug.uptodate

dist/bergamot-translator-worker.wasm: build/bergamot/bergamot-translator-worker.wasm
	mkdir -p dist/
	cp $< $@

dist/bergamot-translator-worker.debug.wasm: build/bergamot-debug/bergamot-translator-worker.wasm
	mkdir -p dist/
	cp $< $@

recompile-bergamot: docker-build-bergamot
	cp --remove-destination dist/bergamot-translator-worker.wasm internal/wasm/bergamot-translator-worker.wasm

gen: internal/wasm/bergamot-translator-worker.wasm
	@go generate ./...

test:
	@go test ./...
