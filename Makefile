REPOSITORY?=  torchbox/k8s-database-controller
TAG?=         latest

build:
	docker build --pull -t ${REPOSITORY}:${TAG} .

push:
	docker push ${REPOSITORY}:${TAG}
