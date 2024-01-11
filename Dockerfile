FROM alpine:3.18
WORKDIR /
RUN mkdir btp-sdk
COPY /build/linux/btp-sdk-cli ./btp-sdk/
CMD ./btp-sdk/btp-sdk-cli server start