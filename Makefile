
tag := $(shell git tag --points-at HEAD )
name := "moshloop/platform-operator"

ifdef tag
else
  tag := latest
endif

.PHONY: release
release: *
	docker build -t docker.io/$(name):$(tag) ./
	docker login -u $(DOCKER_USER) -p $(DOCKER_PASS)
	docker push docker.io/$(name):$(tag)
