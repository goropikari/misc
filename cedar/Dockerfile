FROM rust:1

ARG CEDAR_VERSION=4.11.0

RUN cargo install --locked cedar-policy-cli --version ${CEDAR_VERSION}

WORKDIR /work

CMD ["cedar", "--help"]
