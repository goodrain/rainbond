FROM goodrainapps/alpine:3.4
COPY rainbond-init-probe /run/rainbond-init-probe
RUN chmod 655 /run/rainbond-init-probe
ENV RELEASE_DESC=__RELEASE_DESC__
CMD ["/run/rainbond-init-probe","probe"]

