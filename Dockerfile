FROM golang:alpine AS build
# Working directory will be created if it does not exist
# ENV HOME /src
ENV CGO_ENABLED 0
ENV GOOS linux
ARG VERSION
ARG BUILD_DATE
RUN echo $BUILD_DATE

ARG PKG='github.com/aibotsoft/crypto-collector/pkg'
ARG LDFLAGS='-w -s -X $(PKG)/version.Version=$(VERSION) -X $(PKG)/version.BuildDate=$(BUILD_DATE)'



WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags=${LDFLAGS} -o app main.go

# STAGE 2: build the container to run
FROM gcr.io/distroless/static AS final
#FROM scratch AS final
#WORKDIR /root/
COPY --from=build /src/app /
ENTRYPOINT ["/app"]