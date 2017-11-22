FROM goodrainapps/alpine:3.4

COPY rainbond-worker /run/rainbond-worker
COPY entrypoint.sh /run/entrypoint.sh
RUN chmod 655 /run/rainbond-worker

ENV EX_DOMAIN=ali-sh.goodrain.net:10080
ENV CUR_NET=midonet
ENV RELEASE_DESC=__RELEASE_DESC__

ENTRYPOINT ["/run/entrypoint.sh"]
