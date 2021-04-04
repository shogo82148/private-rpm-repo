TARGZ_FILE := shogo82148.tar.gz
IMAGE_NAME := shogo82148-package

amazonlinux2: IMAGE_NAME := $(IMAGE_NAME)-amazonlinux2
centos7: IMAGE_NAME := $(IMAGE_NAME)-centos7
centos8: IMAGE_NAME := $(IMAGE_NAME)-centos8
almalinux8: IMAGE_NAME := $(IMAGE_NAME)-almalinux8

all: amazonlinux2 centos7 centos8 almalinux8

.PHONY: amazonlinux2
amazonlinux2: amazonlinux2.build

.PHONY: centos7
centos7: centos7.build

.PHONY: centos8
centos8: centos8.build

.PHONY: almalinux8
almalinux8: almalinux8.build

%.build: rpmbuild/SPECS/shogo82148.spec rpmbuild/SOURCES/RPM-GPG-KEY-shogo82148
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
