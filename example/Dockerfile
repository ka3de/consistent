# builder
FROM golang:1.20-alpine

# Set GOPATH so imports in example app
# for consistent library are found
ENV GOPATH=/
# Copy consistent repo to its corresponding
# directory inside GOPATH
COPY . /src/github.com/ka3de/consistent

# Build example app
WORKDIR /src/github.com/ka3de/consistent/example
RUN go build -o /app

# app
FROM alpine:latest
COPY --from=0 /app /app
CMD ["/app"]
