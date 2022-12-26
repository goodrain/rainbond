FROM alpine:3.17
ARG RELEASE_DESC
COPY . /usr/local/bin/
RUN apk update && apk add wget jq bash && \
    wget https://goodrain-delivery.oss-cn-hangzhou.aliyuncs.com/kubectl-linux-$(arch)-1.23.10 -O /usr/bin/kubectl && \
    chmod +x /usr/local/bin/grctl && \
    chmod +x /usr/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl-rbd
ENV RELEASE_DESC=${RELEASE_DESC}
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]