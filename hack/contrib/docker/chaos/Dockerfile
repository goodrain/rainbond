FROM goodrainapps/alpine:3.4

MAINTAINER zengqg@goodrain.com

ENV PKG_URL="http://goodrain-pkg.oss-cn-shanghai.aliyuncs.com/pkg" \
  WORK_DIR=/run

RUN apk --no-cache add openssl git perl openssh-client && \
  curl $PKG_URL/labor_docker.tar.gz | tar -xzC /usr/bin/ && \
  mkdir -p /src

COPY rainbond-chaos entrypoint.sh build.pl /run/
COPY export-app /src/export-app

WORKDIR $WORK_DIR

ENV RELEASE_DESC=__RELEASE_DESC__

ENTRYPOINT ["/run/entrypoint.sh"]
