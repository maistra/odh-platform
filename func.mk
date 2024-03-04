# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
echo "$(LOCALBIN)" ;\
GOBIN=$(LOCALBIN) go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

define go-mod-version
$(shell go mod graph | grep $(1) | head -n 1 | cut -d'@' -f 2)
endef

# Using controller-gen to fetch external CRDs and put them in config/crd/external folder
# They're used in tests, as they have to be created for controller to work
define fetch-external-crds
GOFLAGS="-mod=readonly" $(LOCALBIN)/controller-gen crd \
paths=$(shell go env GOPATH)/pkg/mod/$(1)@$(call go-mod-version,$(1))/$(2)/... \
output:crd:artifacts:config=config/crd/external
endef

# Call this function with $(call header,"Your message") to see underscored green text
define header =
@echo -e "\n\e[92m\e[4m\e[1m$(1)\e[0m\n"
endef
