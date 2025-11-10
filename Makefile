WH_DIR=third_party/whisper.cpp
WH_BUILD=$(WH_DIR)/build

ABS_WH_DIR := $(abspath $(WH_DIR))
ABS_BUILD  := $(abspath $(WH_BUILD))

# try a few likely locations for the vulkan static lib
GGML_VK_LIB := $(firstword \
  $(wildcard $(ABS_BUILD)/ggml/src/libggml-vulkan.a) \
  $(wildcard $(ABS_BUILD)/ggml/src/ggml-vulkan/libggml-vulkan.a) \
  $(wildcard $(ABS_BUILD)/ggml/src/libggml_vk.a) \
)

# fail early if not found
ifeq ($(GGML_VK_LIB),)
$(warning Could not find libggml-vulkan.a; run: find $(ABS_BUILD) -name 'libggml*-vulkan*.a')
endif

COPT ?= -O2 -DNDEBUG
CGO_CFLAGS_COMMON   := $(COPT) -I$(ABS_WH_DIR)/include -I$(ABS_WH_DIR)/ggml/include
CGO_CXXFLAGS_COMMON := $(COPT)

# add -L so trailing -l flags from cgo resolve too
CGO_LDFLAGS_COMMON  := \
	-L$(ABS_BUILD)/src -L$(ABS_BUILD)/ggml/src \
	$(ABS_BUILD)/src/libwhisper.a \
	$(ABS_BUILD)/ggml/src/libggml.a \
	$(GGML_VK_LIB) \
	$(ABS_BUILD)/ggml/src/libggml-base.a \
	$(ABS_BUILD)/ggml/src/libggml-cpu.a \
	-lvulkan -ldl -lpthread -lm -lstdc++

.PHONY: whisper
whisper:
	@test -d $(WH_BUILD) || cmake -S $(WH_DIR) -B $(WH_BUILD) \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF -DWHISPER_BUILD_TESTS=OFF \
		-DGGML_VULKAN=ON -DGGML_CUDA=OFF -DGGML_METAL=OFF -DGGML_SYCL=OFF -DGGML_OPENCL=OFF
	cmake --build $(WH_BUILD) -j

.PHONY: build
build: whisper
	CGO_CFLAGS='$(CGO_CFLAGS_COMMON)' CGO_CXXFLAGS='$(CGO_CXXFLAGS_COMMON)' CGO_LDFLAGS='$(CGO_LDFLAGS_COMMON)' \
		go build -o bin/luch ./cmd/luch
