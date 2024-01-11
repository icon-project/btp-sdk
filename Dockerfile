# syntax=docker/dockerfile:1
FROM alpine:3.18 as build
RUN apk add git make go
RUN mkdir /btp-sdk
COPY . /btp-sdk
WORKDIR /btp-sdk
RUN make linux

FROM alpine:3.18
WORKDIR /
RUN mkdir /btp-sdk
COPY --from=build /btp-sdk/build/linux/btp-sdk-cli /btp-sdk
CMD ./btp-sdk/btp-sdk-cli server start
