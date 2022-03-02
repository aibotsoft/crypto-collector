.EXPORT_ALL_VARIABLES:
SERVICE_NAME=crypto-collector

DOCKER_USERNAME=aibotsoft
GIT_COMMIT?=$(shell git rev-parse "HEAD^{commit}" 2>/dev/null)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null)
BUILD_DATE:=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
PKG:=github.com/$(DOCKER_USERNAME)/$(SERVICE_NAME)/pkg
LDFLAGS:=-w -s -X $(PKG)/version.Version=$(GIT_TAG) -X $(PKG)/version.BuildDate=$(BUILD_DATE)
CGO_ENABLED=0
GOARCH=amd64
VERSION=1.0.0

demo:
	echo ${LDFLAGS}

build:
	CGO_ENABLED=0 GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(SERVICE_NAME).exe main.go

build_linux:
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="$(LDFLAGS)" -o dist/$(SERVICE_NAME) main.go

run:
	go run main.go

test:
	SERVICE_ENV=test
	go test -v -cover ./...

run_build:
	dist/service

#Команды для докера
docker_build:
	docker image build -f Dockerfile -t $$DOCKER_USERNAME/$$SERVICE_NAME .

docker_run_rm:
	docker run --rm -t $$DOCKER_USERNAME/$$SERVICE_NAME

docker_login:
	docker login -u $$DOCKER_USERNAME -p $$DOCKER_PASSWORD

docker_push:
	docker push $$DOCKER_USERNAME/$$SERVICE_NAME

docker_deploy: linux_build docker_build docker_login docker_push
docker_deploy_push_local: docker_build docker_push

#Команды для k8s
kube_deploy:
	kubectl apply -f k8s/

kube_rol:
	kubectl -n micro rollout restart deployment $$SERVICE_NAME

mig_up:
	source .env
	$$DSL=sqlserver://$$MSSQL_USER:$$MSSQL_PASSWORD@$$mssql_host:$$mssql_port?database=$$MSSQL_DATABASE
	migrate -verbose -source file://migrations -database $$DSL goto 2

mig_create:
	migrate create -ext sql -dir migrations vByServices

