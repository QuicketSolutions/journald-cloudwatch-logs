
BINARY := journald-cloudwatch-logs-stretch64
DOCKER_IMAGE := stretch-amd64-golang1.15

LD_FLAGS := -X main.GitCommit="sha1:$(shell git rev-parse HEAD)"
LD_FLAGS += -X main.ReleaseVer="0.2.3+bdwyertech-next"
LD_FLAGS += -X main.ReleaseDate="epoch:@$(shell date -u +%s)"

all: build

docker-image:
	docker build -f Dockerfile -t stretch-amd64--golang1.15 .

build:
	docker run --rm -v $$(pwd):/$$(basename $$(pwd)) $(DOCKER_IMAGE) go build -o $(BINARY) -ldflags "$(LD_FLAGS)"


.PHONY: docker-image build sha1ver buildTime

