
ifndef RELEASE_VERSION
RELEASE_VERSION := dev
endif

ifndef REPO_HOST
REPO_HOST := swr.cn-east-2.myhuaweicloud.com
endif


# build clickhouse-operator
define clickhouse-operator-image
docker build --platform linux/amd64  -t  $(REPO_HOST)/squids/clickhouse-operater:$(RELEASE_VERSION)  -f ./dockerfile/operator/Dockerfile  .
# docker push $(REPO_HOST)/squids/clickhouse-operater:$(RELEASE_VERSION)
endef

# build clickhouse-exporter
define clickhouse-exporter-image
docker build --platform linux/amd64  -t  $(REPO_HOST)/squids/clickhouse-exporter:$(RELEASE_VERSION)  -f ./dockerfile/metrics-exporter/Dockerfile  .
# docker push $(REPO_HOST)/squids/clickhouse-exporter:$(RELEASE_VERSION)
endef

clickhouse-operator:
	$(clickhouse-operator-image)