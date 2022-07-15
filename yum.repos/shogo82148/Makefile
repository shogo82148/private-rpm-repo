TARGZ_FILE := shogo82148.tar.gz
IMAGE_NAME := shogo82148-package

amazonlinux2: IMAGE_NAME := $(IMAGE_NAME)-amazonlinux2
amazonlinux2022: IMAGE_NAME := $(IMAGE_NAME)-amazonlinux2022
centos7: IMAGE_NAME := $(IMAGE_NAME)-centos7
almalinux8: IMAGE_NAME := $(IMAGE_NAME)-almalinux8
almalinux9: IMAGE_NAME := $(IMAGE_NAME)-almalinux9
rockylinux8: IMAGE_NAME := $(IMAGE_NAME)-rockylinux8
rockylinux9: IMAGE_NAME := $(IMAGE_NAME)-rockylinux9

all: amazonlinux2 amazonlinux2022 centos7 almalinux8 almalinux9 rockylinux8 rockylinux9

.PHONY: amazonlinux2
amazonlinux2: amazonlinux2.build

.PHONY: amazonlinux2022
amazonlinux2022: amazonlinux2022.build

.PHONY: centos7
centos7: centos7.build

.PHONY: almalinux8
almalinux8: almalinux8.build

.PHONY: almalinux9
almalinux9: almalinux9.build

.PHONY: rockylinux8
rockylinux8: rockylinux8.build

.PHONY: rockylinux9
rockylinux9: rockylinux9.build

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
