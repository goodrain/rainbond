FROM goodrainapps/alpine:3.4

MAINTAINER zengqg@goodrain.com

RUN apk --no-cache add gcc musl-dev openssl python python-dev py-crypto \
    git openssh-client coreutils perl sudo

ENV PKG_URL="http://goodrain-pkg.oss-cn-shanghai.aliyuncs.com/pkg"

RUN curl $PKG_URL/labor_docker.tar.gz | tar -xzC /usr/bin/ \
    && curl $PKG_URL/labor_libzmq.tar.gz | tar -xzC /usr/local/ \
    && adduser -u 200 -D -S rain \
    && echo 'rain ALL = (root) NOPASSWD: ALL' > /etc/sudoers.d/rain \
    && curl https://bootstrap.pypa.io/get-pip.py | python -

ADD rainbond-chaos /run/rainbond-chaos
ADD entrypoint.sh /run/entrypoint.sh
ADD plugins /run/plugins

ENV REGION_TAG labor
ENV WORK_DIR /run

ENV GR_PIP_VERSION 4
EXPOSE 8888

EXPOSE 9999

VOLUME /data

VOLUME ["/data2","/data3"]

ADD requirements.txt $WORK_DIR/requirements.txt

#RUN pip install pyzmq==16.0.2 --install-option --zmq=/usr/local/libzmq
RUN pip install -i https://pypi.tuna.tsinghua.edu.cn/simple pyzmq==16.0.2 --install-option --zmq=/usr/local/libzmq
RUN pip install -r $WORK_DIR/requirements.txt -i https://pypi.doubanio.com/simple

RUN python -c "import compileall;compileall.compile_dir('$WORK_DIR')" \
    && find $WORK_DIR/plugins -name '*.py' -type f -delete

# fix git warning
ENV HOME=/home/rain
RUN mkdir /home/rain/.config/git/ -pv && \
    touch /home/rain/.config/git/attributes && \
    chown rain /home/rain -R

ENV RELEASE_DESC=__RELEASE_DESC__

WORKDIR $WORK_DIR

USER rain

ENTRYPOINT ["/run/entrypoint.sh"]
