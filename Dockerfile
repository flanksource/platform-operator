# build stage
FROM golang:1.12 AS build-env
# RUN apk --no-cache add build-base git
ADD . /src
RUN cd /src && go build -o platform-operator

# final stage
FROM alpine
COPY --from=build-env /src/platform-operator /
ENTRYPOINT /platform-operator serve
