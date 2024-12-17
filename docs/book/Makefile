# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ROOT_DIR:=$(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
TOOLS_BIN_DIR ?= $(ROOT_DIR)/bin
CRATE_INSTALL := $(realpath ../../third_party/japaric/trust/crate_install.sh)

MDBOOK := $(TOOLS_BIN_DIR)/mdbook
$(MDBOOK):
	$(CRATE_INSTALL) --git rust-lang/mdBook --tag v0.4.10 --to $(TOOLS_BIN_DIR) --force

MDBOOK_TOC := $(TOOLS_BIN_DIR)/mdbook-toc
$(MDBOOK_TOC):
	$(CRATE_INSTALL) --git badboy/mdbook-toc --tag 0.7.0 --to $(TOOLS_BIN_DIR) --force

MDBOOK_MERMAID := $(TOOLS_BIN_DIR)/mdbook-mermaid
$(MDBOOK_MERMAID):
	$(CRATE_INSTALL) --git badboy/mdbook-mermaid --tag v0.8.3 --to $(TOOLS_BIN_DIR) --force
	$(MDBOOK_MERMAID) install

DEPS := $(MDBOOK) $(MDBOOK_TOC) $(MDBOOK_MERMAID)

.PHONY: build
build: $(DEPS)
	$(MDBOOK) build

.PHONY: serve
serve: $(DEPS)
	$(MDBOOK) serve
