FROM alpine:3 as binary
ARG TARGETARCH

RUN apk add --no-cache curl

RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${TARGETARCH}/kubectl" \
  && chmod +x kubectl \
  && curl -L "https://get.helm.sh/helm-v3.17.2-linux-${TARGETARCH}.tar.gz" | tar -xzvf - \
  && mv linux-${TARGETARCH} linux

FROM alpine:3

COPY --from=binary /kubectl /bin/kubectl
COPY --from=binary /linux/helm /bin/helm

ENTRYPOINT [ "sleep", "infinity" ]
