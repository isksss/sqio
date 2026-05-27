FROM golang:1.26-bookworm

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    netcat-openbsd \
    nodejs \
    npm \
    openssh-client \
  && npm install -g markdownlint-cli2@0.18.1 \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace
