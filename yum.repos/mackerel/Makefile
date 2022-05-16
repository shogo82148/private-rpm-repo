TARGZ_FILE := mackerel.tar.gz
IMAGE_NAME := mackerel-package

.PHONY: all
all: amazonlinux2 centos7 almalinux8 rockylinux8

amazonlinux2: IMAGE_NAME := $(IMAGE_NAME)-amazonlinux2
centos7: IMAGE_NAME := $(IMAGE_NAME)-centos7
almalinux8: IMAGE_NAME := $(IMAGE_NAME)-almalinux8
rockylinux8: IMAGE_NAME := $(IMAGE_NAME)-rockylinux8

.PHONY: amazonlinux2
amazonlinux2: amazonlinux2.build

.PHONY: centos7
centos7: centos7.build

.PHONY: almalinux8
almalinux8: almalinux8.build

.PHONY: rockylinux8
rockylinux8: rockylinux8.build

%.build: rpmbuild/SPECS/mackerel.spec rpmbuild/SOURCES/mackerel-rhel.repo rpmbuild/SOURCES/mackerel-amzn2.repo
	[ -d $@.bak ] && rm -rf $@.bak || :
	[ -d $@ ] && mv $@ $@.bak || :
	docker build --file Dockerfile.$* -t $(IMAGE_NAME) .
	docker run --name $(IMAGE_NAME)-tmp $(IMAGE_NAME)
	mkdir -p tmp
	docker wait $(IMAGE_NAME)-tmp
	docker cp $(IMAGE_NAME)-tmp:/tmp/$(TARGZ_FILE) tmp
	docker rm $(IMAGE_NAME)-tmp
	mkdir $@
	tar -xzf tmp/$(TARGZ_FILE) -C $@
	rm -rf tmp Dockerfile
	docker images | grep -q $(IMAGE_NAME) && docker rmi $(IMAGE_NAME) || true
