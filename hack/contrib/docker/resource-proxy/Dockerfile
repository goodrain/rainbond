FROM nginx:1.19
ARG RELEASE_DESC
VOLUME ["/data/nginx/cache"]
ENV RELEASE_DESC=${RELEASE_DESC}

COPY resource-proxy.conf /etc/nginx/conf.d/
ADD docker-entrypoint.sh /run/docker-entrypoint.sh
RUN chmod +x /run/docker-entrypoint.sh
ENTRYPOINT [ "/run/docker-entrypoint.sh" ]
CMD ["nginx", "-g", "daemon off;"]