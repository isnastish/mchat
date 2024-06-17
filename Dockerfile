# Build container
FROM golang:1.22 as build-env

WORKDIR /go/src/github.com/isnastish/chat/services/session
ADD . /go/src/github.com/isnastish/chat/services/session

RUN CGO_ENABLED=0 GOOS=linux go build -a -v -o /go/bin/session github.com/isnastish/chat/services/session

# More about go test command https://pkg.go.dev/cmd/go#hdr-Test_packages
# go test ./... tests all the packages
RUN go test -v -count=1 \
    github.com/isnastish/chat/pkg/commands \
    github.com/isnastish/chat/pkg/validation \
    github.com/isnastish/chat/pkg/backend/memory

# Production container
# We cannot run redis mock inside a docker container, because we would have \
# to install docker inside that container 
FROM golang:1.22 as run-env

COPY --from=build-env /go/bin/session .
COPY --from=build-env /go/src/github.com/isnastish/chat/services/session/*.pem . 

EXPOSE 8080/tcp

ENTRYPOINT [ "./session" ]
CMD [ "--backend", "memory" ]