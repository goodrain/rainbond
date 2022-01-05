ARG BASE_IMAGE_VERSION
FROM rainbond/openresty:${BASE_IMAGE_VERSION}
ARG RELEASE_DESC
ARG GOARCH
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && apk add --no-cache bash net-tools curl tzdata && \
        cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
        echo "Asia/Shanghai" >  /etc/timezone && \
        date && apk del --no-cache tzdata
ADD . /run

RUN set -eux; \
    if [ "${GOARCH}" = "arm64" ]; then \
        wget https://rainbond-pkg.oss-cn-shanghai.aliyuncs.com/5.3-arm/librestychash.so -O /run/nginx/lua/vendor/so/librestychash.so; \
    fi

ENV NGINX_CONFIG_TMPL=/run/nginxtmp
ENV NGINX_CUSTOM_CONFIG=/run/nginx/conf
ENV RELEASE_DESC=${RELEASE_DESC}
ENV OPENRESTY_HOME=/usr/local/openresty
ENV PATH="${PATH}:${OPENRESTY_HOME}/nginx/sbin"
EXPOSE 8080

ENTRYPOINT ["/run/entrypoint.sh"]