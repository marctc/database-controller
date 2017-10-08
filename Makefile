# vim:set sw=8 ts=8 noet:
#
# Copyright (c) 2016-2017 Torchbox Ltd.
#
# Permission is granted to anyone to use this software for any purpose,
# including commercial applications, and to alter it and redistribute it
# freely. This software is provided 'as-is', without any express or implied
# warranty.

REPOSITORY?=  torchbox/k8s-database-controller
TAG?=         latest

build:
	docker build --pull -t ${REPOSITORY}:${TAG} .

push:
	docker push ${REPOSITORY}:${TAG}
