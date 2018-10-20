set -e

printf '####################################################################\n'
printf '>>> Building Docker image.\n\n'
make build TAG=$COMMIT

# If this is a release, push the Docker image to Docker Hub.
if [ "$TRAVIS_PULL_REQUEST" = "false" -a -n "$TRAVIS_TAG" ]; then
	printf '####################################################################\n'
	printf '>>> Creating release.\n\n'

	docker login -u $DOCKER_USER -p $DOCKER_PASSWORD
	docker tag torchbox/k8s-database-controller:$COMMIT torchbox/k8s-database-controller:$TRAVIS_TAG
	docker push torchbox/k8s-database-controller:$TRAVIS_TAG
	docker tag torchbox/k8s-database-controller:$COMMIT torchbox/k8s-database-controller:latest
	docker push torchbox/k8s-database-controller:latest
fi
