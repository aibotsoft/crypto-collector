FROM golang:alpine AS build
# Working directory will be created if it does not exist
# ENV HOME /src
ENV CGO_ENABLED 0
ENV GOOS linux
#ARG VERSION

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build_linux
RUN #go build -ldflags="-s -w -X main.appVersion=${VERSION}" -o app main.go

# STAGE 2: build the container to run
FROM gcr.io/distroless/static AS final
#FROM scratch AS final
#WORKDIR /root/
COPY --from=build /src/app /
ENTRYPOINT ["/app"]