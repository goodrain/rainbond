FROM goodrainapps/alpine:3.4

COPY rainbond_worker /run/rainbond_worker
RUN chmod 655 /run/rainbond_worker

ENV EX_DOMAIN=ali-sh.goodrain.net:10080
ENV CUR_NET=midonet
ENV RELEASE_DESC=__RELEASE_DESC__

CMD  ["/run/rainbond_worker"]
