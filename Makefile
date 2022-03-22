
ifndef RELEASE_VERSION
RELEASE_VERSION := v1.0.0
endif

ifndef REPO_HOST
REPO_HOST :=swr.cn-east-2.myhuaweicloud.com
#REPO_HOST :=registry.cn-hangzhou.aliyuncs.com
endif

ifndef REPO_USER
REPO_USER := squids
#REPO_USER := zyz_k8s
endif

# build clickhouse-operator
define clickhouse-operator-image
docker build --platform linux/amd64  -t  $(REPO_HOST)/$(REPO_USER)/clickhouse-operator:$(RELEASE_VERSION)  -f ./dockerfile/operator/Dockerfile  .
docker push $(REPO_HOST)/$(REPO_USER)/clickhouse-operator:$(RELEASE_VERSION)
endef

# build clickhouse-exporter
define clickhouse-exporter-image
docker build --platform linux/amd64  -t  $(REPO_HOST)/$(REPO_USER)/clickhouse-exporter:$(RELEASE_VERSION)  -f ./dockerfile/metrics-exporter/Dockerfile  .
docker push $(REPO_HOST)/$(REPO_USER)/clickhouse-exporter:$(RELEASE_VERSION)
endef

.PHONY: clickhouse-operator
clickhouse-operator:
	$(clickhouse-operator-image)

.PHONY: clickhouse-exporter
clickhouse-exporter:
	$(clickhouse-exporter-image)
