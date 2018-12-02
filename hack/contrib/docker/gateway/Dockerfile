FROM goodrainapps/alpine:3.4

ADD rainbond-gateway /run/rainbond-gateway
ADD entrypoint.sh /run/entrypoint.sh
ADD openresty-1.13.6.1.tar.gz /
ADD nginxtmp/tmpl /run/nginxtmp
ADD nginx /run/nginx
ADD ingress-controller /ingress-controller

ENV NGINX_CONFIG_TMPL=/run/nginxtmp
ENV NGINX_CUSTOM_CONFIG=/run/nginx/conf
ENV RELEASE_DESC=__RELEASE_DESC__
ENV OPENRESTY_HOME=/usr/local/openresty
ENV PATH="${PATH}:${OPENRESTY_HOME}/nginx/sbin"

EXPOSE 8080

ENTRYPOINT ["/run/entrypoint.sh"]