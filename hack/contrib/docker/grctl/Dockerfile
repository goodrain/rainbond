FROM goodrainapps/alpine:3.4
COPY . /run
RUN chmod +x /run/rainbond-grctl /run/entrypoint.sh
VOLUME [ "/rootfs/root","/rootfs/path","/ssl" ]
ENV RELEASE_DESC=__RELEASE_DESC__
ENTRYPOINT ["/run/entrypoint.sh"]