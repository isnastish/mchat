FROM golang:1.22 as build-env

WORKDIR /go/src/github.com/isnastish/chat/services/session
ADD . /go/src/github.com/isnastish/chat/services/session

RUN go build -v -o /go/bin/session github.com/isnastish/chat/services/session

# We cannot run redis mock inside a docker container, because we would have \
# to install docker inside that container 
FROM golang:1.22 as run-env
COPY --from=build-env /go/bin/session /session/
EXPOSE 8080/tcp

CMD [ "/session/session" ]