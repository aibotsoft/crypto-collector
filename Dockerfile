FROM golang:alpine AS build
# Working directory will be created if it does not exist
ARG LDFLAGS
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -ldflags="$LDFLAGS" -o app main.go

# STAGE 2: build the container to run
FROM gcr.io/distroless/static AS final
COPY --from=build /src/app /
ENTRYPOINT ["/app"]