REPOSITORY?=  kubejam/database-controller
TAG?=         latest

build:
	docker build --pull -t ${REPOSITORY}:${TAG} .

push:
	docker push ${REPOSITORY}:${TAG}
