FROM goodrainapps/alpine:3.4

MAINTAINER zengqg@goodrain.com

RUN apk --no-cache add openssl git perl openssh-client

ENV PKG_URL="http://goodrain-pkg.oss-cn-shanghai.aliyuncs.com/pkg"

RUN curl $PKG_URL/labor_docker.tar.gz | tar -xzC /usr/bin/

ADD rainbond-chaos /run/rainbond-chaos
ADD entrypoint.sh /run/entrypoint.sh
ADD build.pl /run/build.pl

ENV WORK_DIR /run

ENV RELEASE_DESC=__RELEASE_DESC__

WORKDIR $WORK_DIR

ENTRYPOINT ["/run/entrypoint.sh"]
