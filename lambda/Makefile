.PHONY: build
build:
	sam build

.PHONY: deploy
deploy:
	sam deploy \
		--region ap-northeast-1 \
		--stack-name rpm-metadata-updater \
		--image-repository 445285296882.dkr.ecr.ap-northeast-1.amazonaws.com/rpm-metadata-updater \
		--no-fail-on-empty-changeset \
		--capabilities CAPABILITY_IAM
