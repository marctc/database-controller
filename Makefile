REPOSITORY?=  xxxxxxxxxxxx.dkr.ecr.us-east-1.amazonaws.com/database-controller
TAG?=         latest

build:
	docker build --pull -t ${REPOSITORY}:${TAG} .

push:
	docker push ${REPOSITORY}:${TAG}
