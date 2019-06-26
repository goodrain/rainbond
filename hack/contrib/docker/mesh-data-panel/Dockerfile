FROM  envoyproxy/envoy:v1.9.0
LABEL "author"="zengqg@goodrain.com"
RUN apt-get update && apt-get install -y bash curl net-tools wget vim && \
    wget https://github.com/barnettZQG/env2file/releases/download/0.1.1/env2file-linux -O /usr/bin/env2file    
ADD . /root/
RUN chmod 755 /root/start.sh && chmod 755 /usr/bin/env2file
ENV ENVOY_BINARY="/usr/local/bin/envoy"
ENV RELEASE_DESC=__RELEASE_DESC__
WORKDIR /root
CMD ["./start.sh"]



