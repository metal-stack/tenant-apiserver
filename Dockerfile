FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /
COPY bin/server /
CMD ["/server"]
